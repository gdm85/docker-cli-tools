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
	"github.com/gdm85/go-libshell"
	"bufio"
	"fmt"
	"github.com/gdm85/go-dockerclient"
	"github.com/gdm85/goopt"
	"os"
	"sort"
	"strings"
	"time"
)

type ContainerProcessInfo struct {
	ProcessInfo
	Binary        string
	ContainerName string
}

type ProcessInfo struct {
	Pid int
	Cpu float32
}

type SortableProcessInfo []*ProcessInfo

func (s SortableProcessInfo) Len() int {
	return len(s)
}
func (s SortableProcessInfo) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SortableProcessInfo) Less(j, i int) bool {
	return s[i].Cpu < s[j].Cpu
}

var (
	Docker              *docker.Client
	containerNameLookup map[string]string
	headCount           = goopt.Int([]string{"-n", "--number"}, 10, "amount of milliseconds to wait between each sample collection")
	every               = goopt.Int([]string{"-e", "--every"}, 50, "amount of milliseconds to wait between each sample collection")
	maxCollectTime      = goopt.Int([]string{"-t", "--time"}, 1, "amount of seconds to sample data for")
)

func init() {
	var err error
	Docker, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		panic(err)
	}

	containerNameLookup = map[string]string{}
}

func sampleTopData() (SortableProcessInfo, error) {
	result := shell.New("top", "-b", "-n1")
	err := result.Run()
	if err != nil {
		return nil, err
	}

	// proxy the exit code
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("top command execution failed: %s\n", result.Stderr)
	}

	data := []*ProcessInfo{}
	startParsing := false
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	for scanner.Scan() {
		if startParsing {
			var pid, ni, shr int
			var user, virt, res, prio, command, t string
			var status rune
			var cpu, mem float32

			n, err := fmt.Sscanf(scanner.Text(), "%d %s %s %d %s %s %d %c %f %f %s %s\n", &pid, &user, &prio, &ni, &virt, &res, &shr, &status, &cpu, &mem, &t, &command)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed line: %s\n", scanner.Text())
				return nil, err
			}

			if n != 12 {
				return nil, fmt.Errorf("not all fields were read correctly")
			}

			// grab & store
			data = append(data, &ProcessInfo{Pid: pid, Cpu: cpu})
		} else {
			startParsing = strings.HasPrefix(scanner.Text(), "   PID")
		}
	}

	return data, nil
}

func getContainer(pid int) (string, error) {
	inFile, _ := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	defer inFile.Close()
	scanner := bufio.NewScanner(inFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if parts[1] == "perf_event" {
			parts = strings.SplitN(parts[2], "/", 3)
			if parts[1] == "docker" {
				return parts[2], nil
			}
			break
		}
	}
	return "", nil
}

func getContainerName(containerId string) (string, error) {
	if val, ok := containerNameLookup[containerId]; ok {
		return val, nil
	}
	// pull new inspect data from API
	container, err := Docker.InspectContainer(containerId)
	if err != nil {
		return "", err
	}

	containerNameLookup[containerId] = container.Name[1:]

	return container.Name[1:], nil
}

func main() {
	goopt.Description = func() string {
		return "Display biggest CPU consumers over specified timespan."
	}
	goopt.Version = "0.1"
	goopt.Summary = "docker-cpu-killers"
	goopt.Parse(nil)

	data := map[int]float32{}
	takes := 0
	hasToStop := false
	waitChan := make(chan int)

	ticker := time.NewTicker(time.Millisecond * time.Duration(*every))
	go func() {
		for _ = range ticker.C {
			sample, err := sampleTopData()
			if err != nil {
				fmt.Fprintf(os.Stderr, "docker-cpu-killers: %s\n", err.Error())
				waitChan <- 16
				return
			}
			for _, pi := range sample {
				if _, ok := data[pi.Pid]; ok {
					data[pi.Pid] += pi.Cpu
				} else {
					data[pi.Pid] = pi.Cpu
				}
			}
			takes++
			if hasToStop {
				break
			}
		}

		waitChan <- 0
	}()

	time.Sleep(time.Second * time.Duration(*maxCollectTime))
	hasToStop = true

	// wait for ticker loop to exit
	exitCode := <-waitChan
	if exitCode != 0 {
		os.Exit(exitCode)
		return
	}

	// recreate a sortable array
	newSample := SortableProcessInfo{}
	for pid, cpu := range data {
		newSample = append(newSample, &ProcessInfo{Pid: pid, Cpu: cpu / float32(takes)})
	}

	sort.Sort(newSample)
	max := len(newSample)
	if max > *headCount {
		max = *headCount
	}

	// now proceed to show most consuming containers
	output := []*ContainerProcessInfo{}
	selfPid := os.Getpid()
	for _, pi := range newSample {
		if pi.Pid == selfPid {
			continue
		}
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pi.Pid))
		if err != nil {
			if os.IsNotExist(err) {
				// skip
				continue
			}
			fmt.Fprintf(os.Stderr, "docker-cpu-killers: %s\n", err.Error())
			os.Exit(2)
		}

		containerId, err := getContainer(pi.Pid)
		if err != nil {
			if os.IsNotExist(err) {
				// skip
				continue
			}
			fmt.Fprintf(os.Stderr, "docker-cpu-killers: %s\n", err.Error())
			os.Exit(3)
		}

		var containerName string
		if containerId == "" {
			containerName = "?"
		} else {
			containerName, err = getContainerName(containerId)
			if err != nil {
				fmt.Fprintf(os.Stderr, "docker-cpu-killers: %s\n", err.Error())
				os.Exit(4)
			}
		}

		cpi := &ContainerProcessInfo{}
		cpi.Cpu = pi.Cpu
		cpi.Pid = pi.Pid
		cpi.ContainerName = containerName
		cpi.Binary = target

		output = append(output, cpi)
		if len(output) == max {
			break
		}
	}

	maxLen := 0
	for _, pi := range output {
		l := len(pi.ContainerName)
		if l > maxLen {
			maxLen = l
		}
	}
	maxLen++

	for _, pi := range output {
		fmt.Printf("%.2f %9d\t%"+fmt.Sprintf("%d", maxLen)+"s\t%s\n", pi.Cpu, pi.Pid, pi.ContainerName, pi.Binary)
	}
}
