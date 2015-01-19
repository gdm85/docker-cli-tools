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
	"os"
	"regexp"
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
	fmt.Fprintln(os.Stderr, "Usage: docker-grep pattern1 [pattern2] [pattern3] [...] [patternN]")
	fmt.Fprintln(os.Stderr, "docker-grep is part of docker-cli-tools and licensed under GNU GPLv2")
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

func getName(container *docker.APIContainers) (string, error) {
	if inspectData, ok := inspectCache[container.ID]; ok {
		return inspectData.Name, nil
	}

	inspectData, err := fetchInspectData(container.ID)
	if err != nil {
		return "", err
	}

	return inspectData.Name, nil
}

func main() {
	// if no arguments specified, show help and exit with failure
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		showUsage()
		os.Exit(1)
		return
	}

	// containers to filter on
	patterns := os.Args[1:]

	if len(patterns) == 0 {
		fmt.Fprintf(os.Stderr, "docker-grep: no patterns specified\n")
		os.Exit(1)
	}

	// fetch all containers data
	allContainers, err := Docker.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker-grep: %s\n", err)
		os.Exit(1)
	}

	matching := map[string]bool{}
	for _, pattern := range patterns {
		if len(pattern) == 0 {
			fmt.Fprintf(os.Stderr, "docker-grep: empty pattern specified\n")
			os.Exit(1)
		}

		var rx *regexp.Regexp

		for _, container := range allContainers {
			name, err := getName(&container)
			if err != nil {
				fmt.Fprintf(os.Stderr, "docker-grep: %s\n", err)
				os.Exit(1)
			}

			// ignore containers without a name
			if name == "" {
				continue
			}

			// skip already matched names
			if _, seen := matching[name]; seen {
				continue
			}

			if name == pattern {
				matching[name] = true
			} else {
				// as last resort, consider this a regex pattern
				if rx == nil {
					rx, err = regexp.Compile(pattern)
					if err != nil {
						fmt.Fprintf(os.Stderr, "docker-grep: cannot compile regex pattern '%s': %s\n", pattern, err)
						os.Exit(1)
					}
				}
				if rx.MatchString(name) {
					matching[name] = true
					// here we do not break, since multiple matches are allowed for regex patterns
				}
			}

		} // no more container info after this point

		// quit matching if everything was already matched
		if len(matching) == len(allContainers) {
			break
		}
	}

	for name, _ := range matching {
		fmt.Println(name)
	}

	// no matches is still a success
}
