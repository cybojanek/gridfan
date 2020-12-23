package main

/*
Copyright (C) 2018 Jan Kasiak

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

import (
	"fmt"
	"github.com/cybojanek/gridfan/internal/config"
	"github.com/cybojanek/gridfan/internal/controller"
	"github.com/cybojanek/gridfan/internal/daemon"
	"log"
	"os"
	"strconv"
)

func main() {
	// NOTE: we wrap, so that mainWrapper can call defer and clean things up
	//       since calling os.Exit does not honor defered calls
	os.Exit(mainWrapper())
}

func mainWrapper() (ret int) {
	// Default return is error unless we reach end
	ret = 1

	// Check usage
	if !((len(os.Args) == 3 && os.Args[2] == "daemon") ||
		(len(os.Args) == 4 && os.Args[2] == "get") ||
		(len(os.Args) == 5 && os.Args[2] == "set")) {
		fmt.Fprintf(os.Stderr, "Usage: %v\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  YAML_CONFIG_FILE daemon\n")
		fmt.Fprintf(os.Stderr, "  YAML_CONFIG_FILE get all|1|2|3|4|5|6\n")
		fmt.Fprintf(os.Stderr, "  YAML_CONFIG_FILE set all|1|2|3|4|5|6 0|20|21|...|100\n")
		return
	}

	config, err := config.Read(os.Args[1])

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		return
	}

	switch os.Args[2] {

	case "daemon":
		log.Printf("INFO Starting with config: %+v", config)
		daemon.Run(config)

	case "get":
		fallthrough
	case "set":
		// Parse fans
		fans := []int{1, 2, 3, 4, 5, 6}
		if os.Args[3] != "all" {
			fans = fans[0:0]
			value, err := strconv.Atoi(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Bad fan index: %v\n", os.Args[3])
				return
			}
			fans = append(fans, value)
		}

		// Parse rpm
		rpm := 0
		if os.Args[2] == "set" {
			value, err := strconv.Atoi(os.Args[4])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Bad fan RPM: %v\n", os.Args[4])
				return
			}
			rpm = value
		}

		// Open controller
		controller := controller.GridFanController{DevicePath: config.DevicePath}
		if err := controller.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open controller: %v\n", err)
			return
		}

		// Cleanup
		defer func() {
			if err := controller.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to close controller: %v\n", err)
				ret = 1
			}
		}()

		// Run command
		for _, fan := range fans {
			if os.Args[2] == "get" {
				rpm, err := controller.GetSpeed(fan)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to get speed: %v\n", err)
					return
				}
				fmt.Printf("%d %d\n", fan, rpm)
			} else {
				if err := controller.SetSpeed(fan, rpm); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to set speed: %d %d %v\n",
						fan, rpm, err)
					return
				}
			}
		}
	}

	// Exit gracefully
	ret = 0
	return
}
