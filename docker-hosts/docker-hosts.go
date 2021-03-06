/*
 * docker-cli-tools v0.1.0
 * Copyright (C) 2014 gdm85 - https://github.com/gdm85/docker-cli-tools/

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

package main

import (
	"fmt"
	"github.com/gdm85/go-dockerclient"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

var Docker *docker.Client
var inspectCache map[string]*docker.Container

func init() {
	var err error
	Docker, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		panic(err)
	}

	inspectCache = map[string]*docker.Container{}
}

func showUsage() {
	fmt.Fprintln(os.Stderr, "Usage: docker-hosts [container1] [container2] [...] [containerN]")
	fmt.Fprintln(os.Stderr, "docker-hosts is part of docker-cli-tools and licensed under GNU GPLv2")
}

func getNameOrHostname(inspectData *docker.Container) (string, error) {
	name := inspectData.Name
	if len(inspectData.HostnamePath) > 0 {
		bytes, err := ioutil.ReadFile(inspectData.HostnamePath)
		if err != nil {
			return "", err
		}
		hostname := strings.TrimSpace(string(bytes))

		if hostname != name {
			name = fmt.Sprintf("%s (%s)", name, hostname)
		}
	}

	return name, nil
}

func getState(inspectData *docker.Container) string {
	var state string
	if !inspectData.State.Running {
		state = fmt.Sprintf("Exit (%d)", inspectData.State.ExitCode)
	} else {
		if inspectData.State.Paused {
			state = "Paused"
		} else {
			state = "Running"
		}
	}

	return state
}

func display(container *docker.APIContainers) error {
	inspectData, err := fetchInspectData(container.ID)
	if err != nil {
		return err
	}

	nameOrHostname, err := getNameOrHostname(inspectData)
	if err != nil {
		return err
	}

	fmt.Printf("%-40s\t%-23s\t%-10s\t%-16s\n", nameOrHostname, inspectData.Config.Image, getState(inspectData), inspectData.NetworkSettings.IPAddress)
	return nil
}

///
/// fetch inspect data (e.g. all details) and store them in a lookup map
///
func fetchInspectData(ID string) (*docker.Container, error) {
	var inspectData *docker.Container
	var ok bool
	var err error
	if inspectData, ok = inspectCache[ID]; !ok {
		inspectData, err = Docker.InspectContainer(ID)
		if err != nil {
			return nil, err
		}

		// always fix the name leading slash
		inspectData.Name = inspectData.Name[1:]

		inspectCache[ID] = inspectData
	}
	return inspectData, nil
}

func getIDOrName(container *docker.APIContainers) string {
	if inspectData, ok := inspectCache[container.ID]; ok {
		return inspectData.Name
	}

	return container.ID
}

func main() {
	// if no arguments specified, show help and exit with failure
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		showUsage()
		os.Exit(1)
		return
	}

	// fetch all containers data
	allContainers, err := Docker.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker-hosts: %s\n", err)
		os.Exit(1)
	}

	// containers to filter on
	containerIds := os.Args[1:]

	if len(containerIds) == 0 {
		// send all containers to the displayer channel
		for _, container := range allContainers {
			err := display(&container)
			if err != nil {
				fmt.Fprintf(os.Stderr, "docker-hosts: about '%s': %s\n", getIDOrName(&container), err)
				os.Exit(1)
			}
		}
	} else {
		for _, pattern := range containerIds {
			if len(pattern) == 0 {
				fmt.Fprintf(os.Stderr, "docker-hosts: empty container id specified\n")
				os.Exit(1)
			}

			var rx *regexp.Regexp
			var matching []*docker.APIContainers
			var found bool

			for _, container := range allContainers {
				if container.ID == pattern {
					err := display(&container)
					if err != nil {
						fmt.Fprintf(os.Stderr, "docker-hosts: about '%s': %s\n", getIDOrName(&container), err)
						os.Exit(1)
					}
					found = true
					break
				} else if len(pattern) >= 12 && pattern == container.ID[:12] {
					matching = append(matching, &container)
					// stop matching in case of equivocal ids
					if len(matching) > 1 {
						break
					}
				} else {
					// fetch the inspect data to match against the name
					inspectData, err := fetchInspectData(container.ID)
					if err != nil {
						fmt.Fprintf(os.Stderr, "docker-hosts: cannot find '%s': %s\n", getIDOrName(&container), err)
						os.Exit(2)
					}

					if inspectData.Name == pattern {
						err := display(&container)
						if err != nil {
							fmt.Fprintf(os.Stderr, "docker-hosts: about '%s': %s\n", getIDOrName(&container), err)
							os.Exit(1)
						}
						found = true
						break
					}

					// as last resort, consider this a regex pattern
					if rx == nil {
						rx, err = regexp.Compile(pattern)
						if err != nil {
							fmt.Fprintf(os.Stderr, "docker-hosts: cannot compile regex pattern '%s': %s\n", pattern, err)
							os.Exit(1)
						}
					}
					if rx.MatchString(inspectData.Name) {
						err := display(&container)
						if err != nil {
							fmt.Fprintf(os.Stderr, "docker-hosts: about '%s': %s\n", getIDOrName(&container), err)
							os.Exit(1)
						}
						found = true
						// here we do not break, since multiple matches are allowed for regex patterns
					}
				}

			} // no more container info after this point

			if !found {
				if len(matching) > 0 {
					if len(matching) > 1 {
						fmt.Fprintf(os.Stderr, "docker-hosts: id collision match for '%s'\n", pattern)
						os.Exit(2)
					}

					// found matching short id
					container := matching[0]
					err := display(container)
					if err != nil {
						fmt.Fprintf(os.Stderr, "docker-hosts: about '%s': %s\n", getIDOrName(container), err)
						os.Exit(1)
					}
				} // otherwise, nothing matches
			}

			// everything fine, since container was found, continue to next id
		}
	}

}
