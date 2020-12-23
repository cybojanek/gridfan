// Package disk exposes disk information.
package disk

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
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Disk reference.
type Disk struct {
	DevicePath string
}

// Disk status
const (
	DiskStatusSleep = iota
	DiskStatusStandby
	DiskStatusActive
)

////////////////////////////////////////////////////////////////////////////////

// ErrSleepingDisk error.
type ErrSleepingDisk struct {
	message string
}

func (e *ErrSleepingDisk) Error() string {
	return e.message
}

////////////////////////////////////////////////////////////////////////////////

// GetStatusString for status enum
func GetStatusString(status int) string {
	switch status {
	case DiskStatusSleep:
		return "Sleeping"

	case DiskStatusStandby:
		return "Standby"

	case DiskStatusActive:
		return "Active"

	default:
		return "Unknown"
	}
}

////////////////////////////////////////////////////////////////////////////////

// GetTemperature of a disk in degrees celcius.
func (disk *Disk) GetTemperature() (int, error) {

	// Get command output
	command := exec.Command("hddtemp", disk.DevicePath)

	// Save stdout and stderr
	var stdoutBuffer, stderrBuffer bytes.Buffer
	command.Stdout = &stdoutBuffer
	command.Stderr = &stderrBuffer

	err := command.Run()
	stdout := stdoutBuffer.String()
	stderr := stderrBuffer.String()

	if err != nil {
		return 0, err
	}

	// Check for error, since hddtemp returns exit cide 0
	if strings.Contains(stderr, "No such file or directory") {
		return 0, fmt.Errorf("GetTemperature: Disk [%v] not found",
			disk.DevicePath)
	}

	// Check if drive is asleep
	if strings.Contains(stderr, "drive is sleeping") {
		return 0, &ErrSleepingDisk{message: fmt.Sprintf(
			"GetTemperature: Disk [%v] is sleeping", disk.DevicePath)}
	}

	// Split into lines
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		return 0, fmt.Errorf(
			"GetTemperature: Disk [%v] output is not one line: [%v]",
			disk.DevicePath, stdout)
	}

	// Get temperature
	fields := strings.Split(lines[0], ":")
	if len(fields) != 3 {
		return 0, fmt.Errorf(
			"GetTemperature: Disk [%v] output is not three fields: [%v]",
			disk.DevicePath, lines[0])
	}

	field := strings.TrimSpace(fields[2])
	tempStr := field[0:0]
	for i, c := range field {
		if c < '0' || c > '9' {
			break
		}
		tempStr = field[0 : i+1]
	}

	temperature, err := strconv.Atoi(tempStr)
	if err != nil {
		return 0, fmt.Errorf(
			"GetTemperature: Disk [%v] output temperature error: [%v] %v",
			disk.DevicePath, stdout, err)
	}

	return temperature, nil
}

// GetStatus of status of a disk.
func (disk *Disk) GetStatus() (int, error) {
	var status int

	// Get command output
	command := exec.Command("hdparm", "-C", disk.DevicePath)

	// Save stdout and stderr
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return 0, fmt.Errorf(
			"GetStatus: hdparm failed for disk [%v]: stdout:[%v] stderr:[%v] err: %v",
			disk.DevicePath, stdout.String(), stderr.String(), err)
	}
	stringOutput := stdout.String()

	// Split into lines
	lines := strings.Split(strings.TrimSpace(stringOutput), "\n")
	if len(lines) != 2 {
		return 0, fmt.Errorf("GetStatus: output is not two lines: %v",
			stringOutput)
	}

	// NOTE: our notion of standby differs from what hdparm reports...
	statusLine := lines[1]
	switch {
	case strings.Contains(statusLine, "standby"):
		status = DiskStatusSleep

	case strings.Contains(statusLine, "unknown"):
		status = DiskStatusStandby

	case strings.Contains(statusLine, "active/idle"):
		status = DiskStatusActive

	default:
		return 0, fmt.Errorf("GetStatus: bad status line: [%s]", statusLine)
	}

	return status, nil
}
