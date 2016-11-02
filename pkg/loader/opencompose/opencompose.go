/*
Copyright 2016 Skippbox, Ltd All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package opencompose

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"

	"github.com/Sirupsen/logrus"
	"github.com/kubernetes-incubator/kompose/pkg/kobject"

	"gopkg.in/yaml.v2"
)

// Data structure for Service definition
type Service struct {
	Image         string
	ServiceType   string `yaml:"service_type,omitempty"` // clusterIP, loadBalancer, None. http://kubernetes.io/docs/user-guide/services/#publishing-services---service-types
	Ports         []string
	Environment   map[string]string
	ContainerName string
	Entrypoint    []string
	Command       []string
	Volumes       []string
	Build         Build `yaml:"build,omitempty"`
}

// build info for doing image builds using Dockerfile
type Build struct {
	Context    string `yaml:"context,omitempty"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
}

// Data structure for volume definition
type Volume struct {
	Size string `yaml:"size"`
	Mode string `yaml:"mode,omitempty"`
}

// OpenCompose Data structure
type OpenCompose struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
	Volumes  map[string]Volume  `yaml:"volumes,omitempty"`
}

// Load environment variables from compose file
func loadEnvVars(e map[string]string) []kobject.EnvVar {
	envs := []kobject.EnvVar{}
	for k, v := range e {
		envs = append(envs, kobject.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return envs
}

// Load ports from compose file
func loadPorts(composePorts []string) ([]kobject.Ports, error) {
	ports := []kobject.Ports{}
	character := ":"
	for _, port := range composePorts {
		proto := api.ProtocolTCP
		// get protocol
		p := strings.Split(port, "/")
		if len(p) == 2 {
			if strings.EqualFold("tcp", p[1]) {
				proto = api.ProtocolTCP
			} else if strings.EqualFold("udp", p[1]) {
				proto = api.ProtocolUDP
			}
		}
		// port mappings without protocol part
		portNoProto := p[0]
		if strings.Contains(portNoProto, character) {
			hostPort := portNoProto[0:strings.Index(portNoProto, character)]
			hostPort = strings.TrimSpace(hostPort)
			hostPortInt, err := strconv.Atoi(hostPort)
			if err != nil {
				return nil, fmt.Errorf("invalid host port %q", port)
			}
			containerPort := portNoProto[strings.Index(portNoProto, character)+1:]
			containerPort = strings.TrimSpace(containerPort)
			containerPortInt, err := strconv.Atoi(containerPort)
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q", port)
			}
			ports = append(ports, kobject.Ports{
				HostPort:      int32(hostPortInt),
				ContainerPort: int32(containerPortInt),
				Protocol:      proto,
			})
		} else {
			containerPortInt, err := strconv.Atoi(portNoProto)
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q", port)
			}
			ports = append(ports, kobject.Ports{
				ContainerPort: int32(containerPortInt),
				Protocol:      proto,
			})
		}

	}
	return ports, nil
}

func (oc *OpenCompose) LoadFile(serviceFile string) kobject.KomposeObject {
	var opencompose OpenCompose

	// Create an empty kompose internal object
	komposeObject := kobject.KomposeObject{
		ServiceConfigs: make(map[string]kobject.ServiceConfig),
		VolumeConfigs:  make(map[string]kobject.VolumeConfig),
	}

	// Read the opencompose/services file in yaml format and load data in raw bytes
	serviceBytes, err := ioutil.ReadFile(serviceFile)
	if err != nil {
		logrus.Fatalf("Failed to load service file: %v", err)
	}

	// Unmarshall the raw bytes into OpenCompose struct
	if err = yaml.Unmarshal(serviceBytes, &opencompose); err != nil {
		fmt.Errorf("Error Unmarshalling file - %s: %v", serviceFile, err)
	}

	// transform composeObject into komposeObject
	//serviceNames := opencompose.Keys()

	// populate kobject volumeconfigs
	for name, vol := range opencompose.Volumes {
		komposeObject.VolumeConfigs[name] = kobject.VolumeConfig{
			Size: vol.Size,
			Mode: vol.Mode,
		}
	}

	//Populate the kobject
	for name, service := range opencompose.Services {
		serviceConfig := kobject.ServiceConfig{}
		serviceConfig.Image = service.Image
		serviceConfig.ContainerName = service.ContainerName
		serviceConfig.Command = service.Entrypoint
		serviceConfig.Args = service.Command
		serviceConfig.ServiceType = service.ServiceType
		serviceConfig.Volumes = make(map[string]kobject.ServiceVolumes)

		// Load environment
		serviceConfig.Environment = loadEnvVars(service.Environment)

		// Load ports
		ports, err := loadPorts(service.Ports)
		if err != nil {
			logrus.Fatalf("%q failed to load ports from compose file: %v", name, err)
		}
		serviceConfig.Port = ports

		// handle volumes
		for _, vol := range service.Volumes {
			volattr := strings.Split(vol, ":")
			var volname, mountpath string
			var sharedstorage bool
			if len(volattr) > 1 {
				// named volume
				volname, mountpath = volattr[0], volattr[1]
				sharedstorage = true
			} else {
				// unnamed volume
				volname, mountpath = volattr[0], volattr[0]
				sharedstorage = false
			}
			serviceConfig.Volumes[volname] = kobject.ServiceVolumes{
				MountPoint:    mountpath,
				SharedStorage: sharedstorage,
			}
		}

		if service.Build != (Build{}) {
			// Build is given
			imageName := service.Image
			if imageName == "" {
				logrus.Fatalf("Please provide image name.")
			}

			handleBuilds(service.Build, imageName)
		}

		komposeObject.ServiceConfigs[name] = serviceConfig
	}
	return komposeObject
}

func handleBuilds(build Build, imageName string) {

	// FIXME: right now assume that the docker-compose file is in path
	// FIXME: also right now we are not using the build.Dockerfile option
	// docker build -t IMAGE_NAME PATH_TO_DOCKERFILE
	dockerBuild := fmt.Sprintf("docker build -t %s %s", imageName, build.Context)
	err := execute_command(dockerBuild)
	if err != nil {
		logrus.Fatalf("Build Failed due to: %v", err)
	}
	// docker push IMAGE_NAME
	dockerPush := fmt.Sprintf("docker push %s", imageName)
	err = execute_command(dockerPush)
	if err != nil {
		logrus.Fatalf("Image pushing failed: %v", err)
	}
}

func execute_command(command string) error {

	fmt.Printf("Executing: %s\n", command)
	commandSplit := strings.Split(command, " ")

	cmd := exec.Command(commandSplit[0], commandSplit[1:]...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Error creating StdoutPipe for Cmd: %v", err)
	}

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("docker build out | %s\n", scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting Cmd: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("Error waiting for Cmd: %v", err)
	}
	return nil
}
