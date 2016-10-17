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
	"fmt"
	"io/ioutil"
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
	ServiceType   string // clusterIP, loadBalancer, None. http://kubernetes.io/docs/user-guide/services/#publishing-services---service-types
	Ports         []string
	Environment   map[string]string
	ContainerName string
	Entrypoint    []string
	Command       []string
}

// OpenCompose Data structure
type OpenCompose struct {
	Version  string
	Services map[string]Service
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

	//Populate the kobject
	for name, service := range opencompose.Services {
		serviceConfig := kobject.ServiceConfig{}
		serviceConfig.Image = service.Image
		serviceConfig.ContainerName = service.ContainerName
		serviceConfig.Command = service.Entrypoint
		serviceConfig.Args = service.Command
		serviceConfig.ServiceType = service.ServiceType

		// Load environment
		serviceConfig.Environment = loadEnvVars(service.Environment)

		// Load ports
		ports, err := loadPorts(service.Ports)
		if err != nil {
			logrus.Fatalf("%q failed to load ports from compose file: %v", name, err)
		}
		serviceConfig.Port = ports

		komposeObject.ServiceConfigs[name] = serviceConfig
	}

	return komposeObject
}
