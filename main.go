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
	"github.com/cybojanek/gridfan/config"
	"github.com/cybojanek/gridfan/controller"
	"github.com/cybojanek/gridfan/disk"
	"log"
	"os"
	"strconv"
	"time"
)

// PID loop information
// https://en.wikipedia.org/wiki/PID_controller#Pseudocode
type PID struct {
	SetPoint float64

	KP float64
	KI float64
	KD float64

	previousError float64
	integral      float64
	previousTime  time.Time
}

// Reset PID information
func (pid *PID) Reset() {
	pid.previousError = 0
	pid.integral = 0
	pid.previousTime = time.Now()
}

// Update pid information
func (pid *PID) Update(value float64) float64 {
	timeSince := time.Since(pid.previousTime).Seconds()

	error := pid.SetPoint - value
	pid.integral = pid.integral + (error * timeSince)
	derivative := (error - pid.previousError) / (timeSince)
	pid.previousError = error

	return (pid.KP * error) + (pid.KI * pid.integral) + (pid.KD * derivative)
}

// Apply configuration and run indefinitely
func monitorTemperature(config config.Config) {
	diskGroup := disk.DiskGroup{}
	for _, devicePath := range config.DiskControlled.Disks {
		diskGroup.AddDisk(&disk.Disk{DevicePath: devicePath})
	}
	controller := controller.GridFanController{DevicePath: config.DevicePath}

	constantSet := false

	// Default is asleep in case of service restart. This means that if the
	// cooldown did not finish, then the cooldown will be shortened, but we
	// want that to avoid fan spinup on service restart.
	lastStatus := disk.DiskStatusSleep
	deadlineOff := time.Now()
	lastRPM := -1

	// Use PID control
	// FIXME: tune parameters
	pid := PID{
		SetPoint: float64(config.DiskControlled.TargetTemperature),
		KP:       3, KI: 5, KD: 3,
	}

	for {
		// Default is 100 in case of errors
		targetRPM := 100

		// Get disk temperature and status
		temp, tempErr := diskGroup.GetTemperature()
		status, statusErr := diskGroup.GetStatus()
		log.Printf("INFO Temp: %d Status: %d", temp, status)
		if tempErr != nil || statusErr != nil {
			log.Printf("ERROR failed to check disk status: %v %v", tempErr, statusErr)
		} else {
			switch status {

			case disk.DiskStatusSleep:
				// Disks are turned off - turn off fans after a cooldown period
				if lastStatus == disk.DiskStatusSleep {
					timeSince := time.Since(deadlineOff).Seconds()
					if timeSince >= 0 {
						targetRPM = 0
						log.Printf("INFO Disk status is asleep, cooldown finished, setting RPM to: %d",
							targetRPM)
					} else {
						targetRPM = config.DiskControlled.RPM.Sleeping
						log.Printf("INFO Disk status is asleep, cooldown over in: %d, setting RPM to: %d",
							-timeSince, targetRPM)
					}
				} else {
					// Previous status was not asleep
					deadlineOff := time.Until(time.Now().Add(time.Duration(
						config.DiskControlled.SleepingTimeout) * time.Second))
					targetRPM = config.DiskControlled.RPM.Sleeping
					log.Printf("INFO Disks just fell asleep, turning off in: %s, setting RPM to: %d",
						deadlineOff, targetRPM)
				}

			case disk.DiskStatusStandby:
				// Disks are neither fully turned off, and neither active
				// Can't read temperature in this state
				targetRPM = config.DiskControlled.RPM.Standby
				log.Printf("INFO Disk status is standby, setting RPM to: %d", targetRPM)

			case disk.DiskStatusActive:
				// Disks are active - check temperature
				if lastStatus != disk.DiskStatusActive {
					pid.Reset()
				} else {
					pid.Update(float64(temp))
				}

				// FIXME: finish implementing this...
				targetRPM = 50

			default:
				log.Printf("ERROR bad status: %d", status)

			}

			lastStatus = status
		}

		if !constantSet || lastRPM != targetRPM {
			// Open device
			if err := controller.Open(); err != nil {
				log.Printf("ERROR failed to open controller: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// First set constant settings
			if !constantSet {
				log.Printf("INFO setting constant fans: %v", config.ConstantRPM)
				constantSet = true
				for key, value := range config.ConstantRPM {
					if err := controller.SetSpeed(key, value); err != nil {
						log.Printf("ERROR failed to set constant fan speed: %d, %d -> %v",
							key, value, err)
						constantSet = false
					}
				}
			}

			// Set disk fan rpm
			if lastRPM != targetRPM {
				log.Printf("INFO setting disk controlled fans %v to: %d",
					config.DiskControlled.Fans, targetRPM)
				lastRPM = targetRPM
				for _, fan := range config.DiskControlled.Fans {
					if err := controller.SetSpeed(fan, targetRPM); err != nil {
						log.Printf("ERROR failed to set disk fan speed: %d, %d -> %v",
							fan, targetRPM, err)
						lastRPM = -1
					}
				}
			}

			if err := controller.Close(); err != nil {
				log.Printf("ERROR failed to close controller: %v", err)
			}
		} else {
			log.Printf("INFO no RPM change")
		}

		time.Sleep(30 * time.Second)
	}
}

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

	config, err := config.ReadConfig(os.Args[1])

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		return
	}

	switch os.Args[2] {

	case "daemon":
		log.Printf("INFO Starting with config: %+v", config)
		monitorTemperature(config)

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
