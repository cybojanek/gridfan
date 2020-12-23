// Package config handles reading config file.
package config

import (
	"fmt"
	"github.com/cybojanek/gridfan/controller"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// ConfigCurvePoint for a temperature/rpm curve
type ConfigCurvePoint struct {
	Temperature int `yaml:"temp"`
	RPM         int `yaml:"rpm"`
}

// Config for GridFan
type Config struct {
	ConstantRPM map[int]int `yaml:"constant_rpm"`
	CurveFans   []int       `yaml:"curve_fans"`
	DevicePath  string      `yaml:"serial_device_path"`
	Disks       []string    `yaml:"disks"`
	DiskCurve   struct {
		Points          []ConfigCurvePoint `yaml:"points"`
		PollInterval    int                `yaml:"poll_interval"`
		CooldownTimeout int                `yaml:"cooldown_timeout"`
		RPM             struct {
			Sleeping int `yaml:"sleeping"`
			Cooldown int `yaml:"cooldown"`
			Standby  int `yaml:"standby"`
		} `yaml:"rpm"`
	} `yaml:"disk_curve"`
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
	for _, fan := range config.CurveFans {
		if !controller.IsValidFan(fan) {
			return config, fmt.Errorf("ReadConfig: Invalid fan index: %d", fan)
		}

		// Can only be present in one
		_, ok := config.ConstantRPM[fan]
		if ok {
			return config, fmt.Errorf(
				"ReadConfig: Fan %d present in both constant_rpm and curve_rpm", fan)
		}
	}

	// Check Sleeping, Cooldown and Standby
	if !controller.IsValidRPM(config.DiskCurve.RPM.Sleeping) {
		return config, fmt.Errorf("ReadConfig: Invalid sleeping rpm: %d",
			config.DiskCurve.RPM.Sleeping)
	}

	if !controller.IsValidRPM(config.DiskCurve.RPM.Cooldown) {
		return config, fmt.Errorf("ReadConfig: Invalid cooldown rpm: %d",
			config.DiskCurve.RPM.Cooldown)
	}

	if !controller.IsValidRPM(config.DiskCurve.RPM.Standby) {
		return config, fmt.Errorf("ReadConfig: Invalid standby rpm: %d",
			config.DiskCurve.RPM.Standby)
	}

	// Check PollInterval
	if config.DiskCurve.PollInterval < 0 ||
		config.DiskCurve.PollInterval > 3600 {
		return config, fmt.Errorf(
			"ReadConfig: Invalid cooldown_timeout: %d not in [0, 3600]",
			config.DiskCurve.PollInterval)
	}

	// Check CooldownTimeout
	if config.DiskCurve.CooldownTimeout < 0 ||
		config.DiskCurve.CooldownTimeout > 3600 {
		return config, fmt.Errorf(
			"ReadConfig: Invalid cooldown_timeout: %d not in [0, 3600]",
			config.DiskCurve.CooldownTimeout)
	}

	// Check Points
	for i, point := range config.DiskCurve.Points {

		if point.Temperature < 0 || point.Temperature > 100 {
			return config, fmt.Errorf(
				"ReadConfig: Invalid disk_curve temperature: %d not in [0, 100]",
				point.Temperature)
		}

		if i > 0 {
			previousTemperature := config.DiskCurve.Points[i-1].Temperature
			if previousTemperature >= point.Temperature {
				return config, fmt.Errorf(
					"ReadConfig: Invalid disk_curve temperature: %d must be strictly increasing",
					point.Temperature)
			}
		}

		if !controller.IsValidRPM(point.RPM) {
			return config, fmt.Errorf(
				"ReadConfig: Invalid disk_curve rpm: %d", point.RPM)
		}
	}

	return config, nil
}
