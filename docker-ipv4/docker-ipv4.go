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
)

var Docker *docker.Client

func init() {
	var err error
	Docker, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		panic(err)
	}
}

func showUsage() {
	fmt.Fprintln(os.Stderr, "Usage: docker-ipv4 container1 [container2] [...] [containerN]")
	fmt.Fprintln(os.Stderr, "docker-ipv4 is part of docker-cli-tools and licensed under GNU GPLv2")
}

func main() {
	// if no arguments specified, show help and exit with failure
	if len(os.Args) == 1 || (len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
		showUsage()
		os.Exit(1)
		return
	}

	for _, containerId := range os.Args[1:] {
		// pull new inspect data from API
		container, err := Docker.InspectContainer(containerId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docker-ipv4: cannot find '%s': %s\n", containerId, err)
			os.Exit(2)
		}

		if !container.State.Running {
			fmt.Fprintf(os.Stderr, "docker-ipv4: container '%s' is not running\n", container.Name[1:])
			os.Exit(2)
		}

		fmt.Println(container.NetworkSettings.IPAddress)
	}
}
