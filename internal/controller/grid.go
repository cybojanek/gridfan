// Package controller controlls a GridFan fan controller.
package controller

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
	"github.com/tarm/serial"
)

// Controller minimums and maximums
const (
	GridMinFanIndex = 1
	GridMaxFanIndex = 6
	GridMinFanRPM   = 20
	GridMaxFanRPM   = 100
)

// GridFanController for GridFan
type GridFanController struct {
	DevicePath string

	serial *serial.Port
}

////////////////////////////////////////////////////////////////////////////////

// IsValidFan number.
func (controller *GridFanController) IsValidFan(fan int) bool {
	return fan >= GridMinFanIndex && fan <= GridMaxFanIndex
}

// IsValidRPM for a fan.
func (controller *GridFanController) IsValidRPM(rpm int) bool {
	return rpm == 0 || (rpm >= GridMinFanRPM && rpm <= GridMaxFanRPM)
}

////////////////////////////////////////////////////////////////////////////////

// Write all bytes to the serial device
func (controller *GridFanController) writeFully(b []byte) error {
	written := 0
	for written < len(b) {
		n, err := controller.serial.Write(b[written:])
		if err != nil {
			return err
		}
		written += n
	}
	return nil
}

// Read until array is full
func (controller *GridFanController) readFully(b []byte) error {
	read := 0
	for read < len(b) {
		n, err := controller.serial.Read(b[read:])
		if err != nil {
			return err
		}
		read += n
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////

// Open controller
func (controller *GridFanController) Open() error {
	if controller.serial != nil {
		return nil
	}

	c := &serial.Config{Name: controller.DevicePath, Baud: 4800}
	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}
	controller.serial = s

	controller.serial.Flush()

	// Check controller
	if err := controller.Ping(); err != nil {
		// Close, we already have an error...so ignore Close error
		controller.Close()
		return fmt.Errorf("Open: Failed to ping controller: %v", err)
	}

	return nil
}

// Close controller
func (controller *GridFanController) Close() error {
	if controller.serial == nil {
		return nil
	}

	if err := controller.serial.Close(); err != nil {
		return err
	}

	controller.serial = nil

	return nil
}

////////////////////////////////////////////////////////////////////////////////

// Ping controller to check its alive
func (controller *GridFanController) Ping() error {
	if controller.serial == nil {
		return fmt.Errorf("Ping: Controller is not open")
	}

	data := []byte{0xc0}
	if err := controller.writeFully(data); err != nil {
		return err
	}

	reply := make([]byte, 1)
	if err := controller.readFully(reply); err != nil {
		return err
	}

	if reply[0] != 0x21 {
		return fmt.Errorf("Ping: Unexpected reply: %d", reply[0])
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////

// GetSpeed of a fan
func (controller *GridFanController) GetSpeed(fan int) (int, error) {
	speed := 0
	if controller.serial == nil {
		return speed, fmt.Errorf("GetSpeed: Controller is not open")
	}

	if !controller.IsValidFan(fan) {
		return speed, fmt.Errorf(
			"GetSpeed: Bad fan number: %d not in range [%d, %d]", fan,
			GridMinFanIndex, GridMaxFanIndex)
	}

	data := []byte{0x8a, byte(fan)}
	if err := controller.writeFully(data); err != nil {
		return speed, err
	}

	reply := make([]byte, 5)
	if err := controller.readFully(reply); err != nil {
		return speed, err
	}

	if !bytes.Equal(reply[0:3], []byte{0xc0, 0x00, 0x00}) {
		return speed, fmt.Errorf("GetSpeed: Malformed reply: %v", reply)
	}

	speed = (int(reply[3]) << 8) | int(reply[4])

	return speed, nil
}

// SetSpeed of a fan
func (controller *GridFanController) SetSpeed(fan int, rpm int) error {
	if controller.serial == nil {
		return fmt.Errorf("SetSpeed: Controller is not open")
	}

	if !controller.IsValidFan(fan) {
		return fmt.Errorf("SetSpeed: Bad fan number: %d not in range [%d, %d]",
			fan, GridMinFanIndex, GridMaxFanIndex)
	}

	if !controller.IsValidRPM(rpm) {
		return fmt.Errorf("SetSpeed: Bad fan rpm: %d not in range [%d, %d]",
			rpm, GridMinFanRPM, GridMaxFanRPM)
	}

	byteA := byte(0)
	byteB := byte(0)
	if rpm != 0 {
		byteA = 0x2 + (byte(rpm) / 10)
		byteB = (byte(rpm) % 10) * 0x10
	}
	data := []byte{0x44, byte(fan), 0xc0, 0x00, 0x00, byteA, byteB}

	if err := controller.writeFully(data); err != nil {
		return err
	}

	reply := make([]byte, 1)
	if err := controller.readFully(reply); err != nil {
		return err
	}

	if reply[0] != 0x1 {
		return fmt.Errorf("SetSpeed: Unexpected reply: %d", reply[0])
	}

	return nil
}
