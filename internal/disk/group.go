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

// DiskGroup of disks
type DiskGroup struct {
	Disks []*Disk
}

// Add a disk to the group.
func (group *DiskGroup) AddDisk(disk *Disk) {
	group.Disks = append(group.Disks, disk)
}

// GetTemperature maximum of all disks
func (group *DiskGroup) GetTemperature() (int, error) {
	maxTemperature := 0

	for _, disk := range group.Disks {

		temperature, err := disk.GetTemperature()

		if err != nil {
			switch err.(type) {
			case *ErrSleepingDisk:
				continue

			default:
				return maxTemperature, err
			}
		}

		if temperature > maxTemperature {
			maxTemperature = temperature
		}
	}

	return maxTemperature, nil
}

// GetStatus of highest activity disk
func (group *DiskGroup) GetStatus() (int, error) {
	maxStatus := DiskStatusSleep

	for _, disk := range group.Disks {

		status, err := disk.GetStatus()

		if err != nil {
			return status, err
		}

		if status > maxStatus {
			maxStatus = status
		}
	}

	return maxStatus, nil
}
