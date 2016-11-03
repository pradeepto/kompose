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
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"

	"github.com/Sirupsen/logrus"
	"github.com/kubernetes-incubator/kompose/pkg/kobject"

	"gopkg.in/yaml.v2"
)

// Data structure for Service definition
type Service struct {
	Image         string            `yaml:"image"`
	ServiceType   string            `yaml:"type,omitempty"` // clusterIP, loadBalancer, None. http://kubernetes.io/docs/user-guide/services/#publishing-services---service-types
	Ports         []string          `yaml:"ports,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Entrypoint    []string          `yaml:"entrypoint,omitempty"`
	Command       []string          `yaml:"command,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Build         Build             `yaml:"build,omitempty"`
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

func randomPortGenerator() int {
	max := 65535
	min := 31000
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(max-min) + min
}

// Load ports from compose file
func loadPorts(ports []string) ([]kobject.Ports, error) {
	k8sports := []kobject.Ports{}

	for _, port := range ports {
		p := strings.Split(port, ":")
		var proto api.Protocol
		var hostPort, containerPort int
		var err error

		switch len(p) {
		case 1:
			proto = api.ProtocolTCP

			// 3306
			hostPort, err := strconv.Atoi(p[0])
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q", p[0])
			}
			containerPort = hostPort

		case 2:
			if strings.EqualFold("tcp", p[0]) {
				// tcp:3306
				proto = api.ProtocolTCP

				hostPort, err := strconv.Atoi(p[1])
				if err != nil {
					return nil, fmt.Errorf("invalid container port %q", p[1])
				}
				containerPort = hostPort

			} else if strings.EqualFold("udp", p[0]) {
				// udp:16667
				proto = api.ProtocolUDP

				hostPort, err := strconv.Atoi(p[1])
				if err != nil {
					return nil, fmt.Errorf("invalid container port %q", p[1])
				}
				containerPort = hostPort

			} else {
				// 13306:3306
				proto = api.ProtocolTCP

				hostPort, err = strconv.Atoi(p[0])
				if err != nil {
					return nil, fmt.Errorf("invalid container port %q", p[0])
				}

				containerPort, err = strconv.Atoi(p[1])
				if err != nil {
					return nil, fmt.Errorf("invalid container port %q", p[1])
				}

			}
		case 3:
			if strings.EqualFold("tcp", p[0]) || len(p[0]) == 0 {
				// tcp:13306:3306
				// ::3306
				proto = api.ProtocolTCP
			} else if strings.EqualFold("udp", p[0]) {
				// udp:16667:6667
				proto = api.ProtocolUDP
			} else {
				return nil, fmt.Errorf("invalid protocol %q", p[0])
			}
			// tcp::3306
			hostPort, err = strconv.Atoi(p[1])
			if err != nil && len(p[1]) != 0 {
				return nil, fmt.Errorf("invalid container port %q", p[1])
			} else if len(p[1]) == 0 {
				hostPort = randomPortGenerator()
			}

			containerPort, err = strconv.Atoi(p[2])
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q", p[2])
			}
		}
		k8sports = append(k8sports, kobject.Ports{
			HostPort:      int32(hostPort),
			ContainerPort: int32(containerPort),
			Protocol:      proto,
		})
	}
	return k8sports, nil
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
		serviceConfig.Annotations = make(map[string]string)

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
		// Configure service types
		switch strings.ToLower(service.ServiceType) {
		case "", "internal":
			serviceConfig.Annotations["kompose.service.type"] = string(api.ServiceTypeClusterIP)
		case "external":
			serviceConfig.Annotations["kompose.service.type"] = string(api.ServiceTypeLoadBalancer)
		default:
			logrus.Fatalf("Unknown service_type value '%s', supported values are 'internal' and 'external'", service.ServiceType)
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
