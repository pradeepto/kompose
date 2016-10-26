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

package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/lookup"
	"github.com/docker/libcompose/project"
	"github.com/kubernetes-incubator/kompose/pkg/kobject"
)

type Compose struct {
}

// load environment variables from compose file
func loadEnvVars(envars []string) []kobject.EnvVar {
	envs := []kobject.EnvVar{}
	for _, e := range envars {
		character := ""
		equalPos := strings.Index(e, "=")
		colonPos := strings.Index(e, ":")
		switch {
		case equalPos == -1 && colonPos == -1:
			character = ""
		case equalPos == -1 && colonPos != -1:
			character = ":"
		case equalPos != -1 && colonPos == -1:
			character = "="
		case equalPos != -1 && colonPos != -1:
			if equalPos > colonPos {
				character = ":"
			} else {
				character = "="
			}
		}

		if character == "" {
			envs = append(envs, kobject.EnvVar{
				Name: e,
			})
		} else {
			values := strings.SplitN(e, character, 2)
			envs = append(envs, kobject.EnvVar{
				Name:  values[0],
				Value: values[1],
			})
		}
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

// load compose file into KomposeObject
func (c *Compose) LoadFile(file string) kobject.KomposeObject {
	komposeObject := kobject.KomposeObject{
		ServiceConfigs: make(map[string]kobject.ServiceConfig),
		VolumeConfigs:  make(map[string]kobject.VolumeConfig),
	}
	context := &project.Context{}
	if file == "" {
		file = "docker-compose.yml"
	}
	context.ComposeFiles = []string{file}

	if context.ResourceLookup == nil {
		context.ResourceLookup = &lookup.FileResourceLookup{}
	}

	if context.EnvironmentLookup == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return kobject.KomposeObject{}
		}
		context.EnvironmentLookup = &lookup.ComposableEnvLookup{
			Lookups: []config.EnvironmentLookup{
				&lookup.EnvfileLookup{
					Path: filepath.Join(cwd, ".env"),
				},
				&lookup.OsEnvLookup{},
			},
		}
	}

	// load compose file into composeObject
	composeObject := project.NewProject(context, nil, nil)
	err := composeObject.Parse()
	if err != nil {
		logrus.Fatalf("Failed to load compose file: %v", err)
	}

	// transform composeObject into komposeObject
	composeServiceNames := composeObject.ServiceConfigs.Keys()

	// volume config and network config are not supported
	if len(composeObject.NetworkConfigs) > 0 {
		logrus.Warningf("Unsupported network configuration of compose v2 - ignoring")
	}
	if len(composeObject.VolumeConfigs) > 0 {
		logrus.Warningf("Unsupported volume configuration of compose v2 - ignoring")
	}

	networksWarningFound := false

	for _, name := range composeServiceNames {
		if composeServiceConfig, ok := composeObject.ServiceConfigs.Get(name); ok {
			//FIXME: networks always contains one default element, even it isn't declared in compose v2.
			if composeServiceConfig.Networks != nil && len(composeServiceConfig.Networks.Networks) > 0 &&
				composeServiceConfig.Networks.Networks[0].Name != "default" &&
				!networksWarningFound {
				logrus.Warningf("Unsupported key networks - ignoring")
				networksWarningFound = true
			}
			kobject.CheckUnsupportedKey(composeServiceConfig)
			serviceConfig := kobject.ServiceConfig{}
			serviceConfig.Image = composeServiceConfig.Image
			serviceConfig.ContainerName = composeServiceConfig.ContainerName
			serviceConfig.Command = composeServiceConfig.Entrypoint
			serviceConfig.Args = composeServiceConfig.Command
			serviceConfig.Volumes = make(map[string]kobject.ServiceVolumes)

			envs := loadEnvVars(composeServiceConfig.Environment)
			serviceConfig.Environment = envs

			// load ports
			ports, err := loadPorts(composeServiceConfig.Ports)
			if err != nil {
				logrus.Fatalf("%q failed to load ports from compose file: %v", name, err)
			}
			serviceConfig.Port = ports

			serviceConfig.WorkingDir = composeServiceConfig.WorkingDir

			if composeServiceConfig.Volumes != nil {
				var count int
				for _, volume := range composeServiceConfig.Volumes.Volumes {
					volname, hostPath, mountPath, mode, err := ParseVolume(volume.String())
					if err != nil {
						logrus.Warningf("Failed to configure container volume: %v", err)
						continue
					}
					if volname == "" {
						volname = fmt.Sprintf("%s-claim%d", name, count)
						count++
					}

					serviceConfig.Volumes[volname] = kobject.ServiceVolumes{
						MountPoint:    mountPath,
						SharedStorage: true,
					}

					// check if ro/rw mode is defined, default rw
					readonly := len(mode) > 0 && mode == "ro"
					if readonly {
						mode = "ReadOnlyMany"
					} else {
						mode = "ReadWriteOnce"
					}

					komposeObject.VolumeConfigs[volname] = kobject.VolumeConfig{
						Size: "100Mi",
						Mode: mode,
					}
					if len(hostPath) > 0 {
						logrus.Warningf("Volume mount on the host %q isn't supported - ignoring path on the host", hostPath)
					}

				}
			}
			// convert compose labels to annotations
			serviceConfig.Annotations = map[string]string(composeServiceConfig.Labels)

			serviceConfig.CPUSet = composeServiceConfig.CPUSet
			serviceConfig.CPUShares = int64(composeServiceConfig.CPUShares)
			serviceConfig.CPUQuota = int64(composeServiceConfig.CPUQuota)
			serviceConfig.CapAdd = composeServiceConfig.CapAdd
			serviceConfig.CapDrop = composeServiceConfig.CapDrop
			serviceConfig.Expose = composeServiceConfig.Expose
			serviceConfig.Privileged = composeServiceConfig.Privileged
			serviceConfig.Restart = composeServiceConfig.Restart
			serviceConfig.User = composeServiceConfig.User
			serviceConfig.VolumesFrom = composeServiceConfig.VolumesFrom

			komposeObject.ServiceConfigs[name] = serviceConfig
		}
	}

	return komposeObject
}

// parseVolume parse a given volume, which might be [name:][host:]container[:access_mode]
func ParseVolume(volume string) (name, host, container, mode string, err error) {
	separator := ":"
	volumeStrings := strings.Split(volume, separator)
	if len(volumeStrings) == 0 {
		return
	}
	// Set name if existed
	if !isPath(volumeStrings[0]) {
		name = volumeStrings[0]
		volumeStrings = volumeStrings[1:]
	}
	if len(volumeStrings) == 0 {
		err = fmt.Errorf("invalid volume format: %s", volume)
		return
	}
	if volumeStrings[len(volumeStrings)-1] == "rw" || volumeStrings[len(volumeStrings)-1] == "ro" {
		mode = volumeStrings[len(volumeStrings)-1]
		volumeStrings = volumeStrings[:len(volumeStrings)-1]
	}
	container = volumeStrings[len(volumeStrings)-1]
	volumeStrings = volumeStrings[:len(volumeStrings)-1]
	if len(volumeStrings) == 1 {
		host = volumeStrings[0]
	}
	if !isPath(container) || (len(host) > 0 && !isPath(host)) || len(volumeStrings) > 1 {
		err = fmt.Errorf("invalid volume format: %s", volume)
		return
	}
	return
}

func isPath(substring string) bool {
	return strings.Contains(substring, "/")
}
