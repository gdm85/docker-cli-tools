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
	"github.com/fsouza/go-dockerclient"
	"os"
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
}

func showUsage() {
	fmt.Fprintln(os.Stderr, "Usage: docker-images [partial-match]")
	fmt.Fprintln(os.Stderr, "docker-images is part of docker-cli-tools and licensed under GNU GPLv2")
}

func getNamesOrIDs(image *docker.APIImages) []string {
	buffer := []string{}
	for _, name := range image.RepoTags {
		if name == "<none>:<none>" {
			buffer = append(buffer, image.ID)
		} else {
			buffer = append(buffer, name)
		}
	}

	return buffer
}

func display(image *docker.APIImages, name string) {
	fmt.Println(name)
}

func main() {
	// if no arguments specified, show help and exit with failure
	if len(os.Args) > 2 || (len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
		showUsage()
		os.Exit(1)
		return
	}

	// fetch all containers data
	allImages, err := Docker.ListImages(docker.ListImagesOptions{All: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker-images: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) == 2 {
		// show images that have at least a partial pattern match
		pattern := os.Args[1]

		for _, image := range allImages {
			for _, name := range getNamesOrIDs(&image) {
				if strings.Contains(name, pattern) {
					display(&image, name)
				}
			}
		}
	} else {
		// show all images
		for _, image := range allImages {
			for _, name := range getNamesOrIDs(&image) {
				display(&image, name)
			}
		}
	}
}
