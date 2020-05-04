// Package config handles reading config file.
package config

import (
	"fmt"
	"github.com/cybojanek/gridfan/controller"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// Config for GridFan
type Config struct {
	DevicePath     string      `yaml:"serial_device_path"`
	ConstantRPM    map[int]int `yaml:"constant_rpm"`
	DiskControlled struct {
		Fans              []int    `yaml:"fans"`
		TargetTemperature int      `yaml:"target_temp"`
		SleepingTimeout   int      `yaml:"sleeping_timeout"`
		Disks             []string `yaml:"disks"`
		RPM               struct {
			Sleeping int `yaml:"sleeping"`
			Standby  int `yaml:"standby"`
		} `yaml:"rpm"`
	} `yaml:"disk_controlled"`
}

// Read yaml config file
func ReadConfig(path string) (Config, error) {
	config := Config{}
	controller := controller.GridFanController{}

	// Read config file
	configContents, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		return config, err
	}

	// yaml decode
	err = yaml.Unmarshal(configContents, &config)
	if err != nil {
		return config, err
	}

	// Check DevicePath
	if len(config.DevicePath) == 0 {
		return config, fmt.Errorf("ReadConfig: Missing serial_device_path")
	}

	// Check ConstantRPM fans
	for fan, rpm := range config.ConstantRPM {
		if !controller.IsValidFan(fan) {
			return config, fmt.Errorf("ReadConfig: Invalid fan index: %d", fan)
		}
		if !controller.IsValidRPM(rpm) {
			return config, fmt.Errorf(
				"ReadConfig: Invalid fan %d rpm: %d", fan, rpm)
		}
	}

	// Check DiskControlled.Fans
	for _, fan := range config.DiskControlled.Fans {
		if !controller.IsValidFan(fan) {
			return config, fmt.Errorf("ReadConfig: Invalid fan index: %d", fan)
		}

		// Can only be present in one
		_, ok := config.ConstantRPM[fan]
		if ok {
			return config, fmt.Errorf(
				"ReadConfig: Fan %d present in both constant_rpm and disk_controlled", fan)
		}
	}

	// Check Sleeping and Standby
	if !controller.IsValidRPM(config.DiskControlled.RPM.Sleeping) {
		return config, fmt.Errorf("ReadConfig: Invalid sleeping rpm: %d",
			config.DiskControlled.RPM.Sleeping)
	}

	if !controller.IsValidRPM(config.DiskControlled.RPM.Standby) {
		return config, fmt.Errorf("ReadConfig: Invalid standby rpm: %d",
			config.DiskControlled.RPM.Standby)
	}

	// Check SleepingTimeout
	if config.DiskControlled.SleepingTimeout < 0 ||
		config.DiskControlled.SleepingTimeout > 3600 {
		return config, fmt.Errorf(
			"ReadConfig: Invalid sleeping_timeout: %d not in [0, 3600]",
			config.DiskControlled.SleepingTimeout)
	}

	// Check TargetTemperature
	if config.DiskControlled.TargetTemperature < 0 ||
		config.DiskControlled.TargetTemperature > 100 {
		return config, fmt.Errorf(
			"ReadConfig: Invalid target_temp: %d not in [0, 100]",
			config.DiskControlled.TargetTemperature)
	}

	return config, nil
}
