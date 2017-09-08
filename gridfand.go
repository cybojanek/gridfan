package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
    "github.com/tarm/serial"
)

const MinFanIndex = 1
const MaxFanIndex = 6
const MinFanRPM = 20
const MaxFanRPM = 100

const (
    DISK_STATUS_ASLEEP = iota
    DISK_STATUS_STANDBY
    DISK_STATUS_ACTIVE
)

type DiskGroup struct {
    Disks []string
}

type FanController struct {
    DevicePath string

    serial *serial.Port
}

type Config struct {
    DevicePath string       `json:"serial_device_path"`
    ConstantRPM map[int]int `json:"constant_rpm"`
    DiskControlled struct {
        Fans []int            `json:"fans"`
        TargetTemperature int `json:"target_temp"`
        SleepingTimeout int   `json:"sleeping_timeout"`
        Disks []string        `json:"disks"`
        RPM struct {
            Sleeping int    `json:"sleeping"`
            Standby int     `json:"standby"`
        } `json:"rpm"`
    } `json:"disk_controlled"`
}

func min(a int, b int) (int) {
    if a < b {
        return a
    } else {
        return b
    }
}

func max(a int, b int) (int) {
    if a > b {
        return a
    } else {
        return b
    }
}

// Get the maximum disk temperature of a list of disk
func (diskGroup DiskGroup) GetTemperature() (int, error) {
    maxTemperature := 0
    for _, disk := range diskGroup.Disks {

        // Get command output
        command := exec.Command("hddtemp", disk)

        // Save stdout and stderr
        var stdout, stderr bytes.Buffer
        command.Stdout = &stdout
        command.Stderr = &stderr

        if err := command.Run(); err != nil {
            log.Printf("ERROR hddtemp failed for disk [%v]: stdout:[%v] stderr:[%v] err: %v",
                disk, stdout.String(), stderr.String(), err)
            return maxTemperature, err
        }
        stringOutput := stdout.String()

        // Split into lines
        lines := strings.Split(strings.TrimSpace(stringOutput), "\n")
        if len(lines) != 1 {
            return maxTemperature, errors.New(fmt.Sprintf("output is not one line: %v",
                stringOutput))
        }

        // Check for error, since hddtemp returns exit cide 0
        if strings.Contains(lines[0], "No such file or directory") {
            return maxTemperature, errors.New(fmt.Sprintf("Disk [%v] not found", disk))
        }

        // Check if drive is asleep
        if strings.Contains(lines[0], "drive is sleeping") {
            return 0, nil
        }

        // Get temperature
        fields := strings.Split(lines[0], ":")
        if len(fields) != 3 {
            return maxTemperature, errors.New(fmt.Sprintf("output is not three fields: %v",
                lines[0]))
        }

        field := strings.TrimSpace(fields[2])
        tempStr := field[0:0]
        for i, c := range field {
            if c < '0' || c > '9' {
                break
            }
            tempStr = field[0:i + 1]
        }

        if temperature, err := strconv.Atoi(tempStr); err != nil {
            return maxTemperature, err
        } else  {
            maxTemperature = max(temperature, maxTemperature)
        }
    }
    return maxTemperature, nil
}

// Get the highest activity status of a list of disks
func (diskGroup DiskGroup) GetStatus() (int, error) {
    status := DISK_STATUS_ASLEEP
    for _, disk := range diskGroup.Disks {
        // Get command output
        command := exec.Command("hdparm", "-C", disk)

        // Save stdout and stderr
        var stdout, stderr bytes.Buffer
        command.Stdout = &stdout
        command.Stderr = &stderr

        if err := command.Run(); err != nil {
            log.Printf("ERROR hdparm failed for disk [%v]: stdout:[%v] stderr:[%v] err: %v",
                disk, stdout.String(), stderr.String(), err)
            return status, err
        }
        stringOutput := stdout.String()

        // Split into lines
        lines := strings.Split(strings.TrimSpace(stringOutput), "\n")
        if len(lines) != 2 {
            return status, errors.New(fmt.Sprintf("output is not two lines: %v",
                stringOutput))
        }

        // NOTE: our notion of standby differs from what hdparm reports...
        status_line := lines[1]
        switch {
        case strings.Contains(status_line, "standby"):
            status = max(status, DISK_STATUS_ASLEEP)

        case strings.Contains(status_line, "unknown"):
            status = max(status, DISK_STATUS_STANDBY)

        case strings.Contains(status_line, "active/idle"):
            status = max(status, DISK_STATUS_ACTIVE)

        default:
            return status, errors.New(fmt.Sprintf("bad status line: [%s]", status_line))
        }
    }
    return status, nil
}

// Write all bytes to the serial device
func (controller *FanController) writeFully(b []byte) (error) {
    written := 0
    for written < len(b) {
        if n, err := controller.serial.Write(b[written:]); err != nil {
            return err
        } else {
            written += n
        }
    }
    return nil
}

// Read until array is full
func (controller *FanController) readFully(b []byte) (error) {
    read := 0
    for read < len(b) {
        if n, err := controller.serial.Read(b[read:]); err != nil {
            return err
        } else {
            read += n
        }
    }
    return nil
}

// Open controller
func (controller *FanController) Open() (error) {
    if controller.serial != nil {
        return nil
    }

    c := &serial.Config{Name: controller.DevicePath, Baud: 4800}
    if s, err := serial.OpenPort(c); err != nil {
        return err
    } else {
        controller.serial = s
    }

    controller.serial.Flush()

    // Check controller
    if err := controller.Ping(); err != nil {
        // Close, we already have an error...so ignore Close error
        controller.Close()
        return errors.New(fmt.Sprintf("Failed to ping controller: %v\n", err))
    }

    return nil
}

// Close controller
func (controller *FanController) Close() (error) {
    if controller.serial == nil {
        return nil
    }

    if err := controller.serial.Close(); err != nil {
        return err
    }

    controller.serial = nil

    return nil
}

// Ping controller to check its alive
func (controller *FanController) Ping() (error) {
    if controller.serial == nil {
        return errors.New(fmt.Sprintf("Controller is not open"))
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
        return errors.New(fmt.Sprintf("Unexpected reply: %d", reply[0]))
    }

    return nil
}

// Get the speed of a fan
func (controller *FanController) GetSpeed(fan int) (int, error) {
    speed := 0
    if controller.serial == nil {
        return speed, errors.New(fmt.Sprintf("Controller is not open"))
    }

    if !isValidFan(fan) {
        return speed, errors.New(fmt.Sprintf("Bad fan number: %d", fan))
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
        return speed, errors.New(fmt.Sprintf("Malformed reply: %v", reply))
    }

    speed = (int(reply[3]) << 8) | int(reply[4])

    return speed, nil
}

// Set the speed of a fan
func (controller *FanController) SetSpeed(fan int, rpm int) (error) {
    if controller.serial == nil {
        return errors.New(fmt.Sprintf("Controller is not open"))
    }

    if !isValidFan(fan) {
        return errors.New(fmt.Sprintf("Bad fan number: %d", fan))
    }

    if !isValidRPM(rpm) {
        return errors.New(fmt.Sprintf("Bad fan rpm: %d", rpm))
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
        return errors.New(fmt.Sprintf("Unexpected reply: %d", reply[0]))
    }

    return nil
}

// Check if a fan number is valid
func isValidFan(fan int) (bool) {
    return fan >= MinFanIndex && fan <= MaxFanIndex
}

// Check if a fan RPM is valid
func isValidRPM(rpm int) (bool) {
    return rpm == 0 || (rpm >= MinFanRPM && rpm <= MaxFanRPM)
}

// Read JSON config file
func readConfig(path string) (Config, error) {
    config := Config{}

    // Read config file
    configContents, err := ioutil.ReadFile(os.Args[1])
    if err != nil {
        return config, err
    }

    // JSON decode
    err = json.Unmarshal(configContents, &config)
    if err != nil {
        return config, err
    }

    // Check DevicePath
    if len(config.DevicePath) == 0 {
        return config, errors.New(fmt.Sprintf("Missing serial_device_path"))
    }

    // Check ConstantRPM fans
    for fan, rpm := range config.ConstantRPM {
        if !isValidFan(fan) {
            return config, errors.New(fmt.Sprintf("Invalid fan index: %d", fan))
        }
        if !isValidRPM(rpm) {
            return config, errors.New(fmt.Sprintf("Invalid fan %d rpm: %d", fan, rpm))
        }
    }

    // Check DiskControlled.Fans
    for _, fan := range config.DiskControlled.Fans {
        if !isValidFan(fan) {
            return config, errors.New(fmt.Sprintf("Invalid fan index: %d", fan))
        }

        // Can only be present in one
        _, ok := config.ConstantRPM[fan]
        if ok {
            return config, errors.New(fmt.Sprintf(
                "Fan %d present in both constant_rpm and disk_controlled", fan))
        }
    }

    // Check Sleeping and Standby
    if !isValidRPM(config.DiskControlled.RPM.Sleeping) {
            return config, errors.New(fmt.Sprintf("Invalid sleeping rpm: %d",
                config.DiskControlled.RPM.Sleeping))
    }

    if !isValidRPM(config.DiskControlled.RPM.Standby) {
            return config, errors.New(fmt.Sprintf("Invalid standby rpm: %d",
                config.DiskControlled.RPM.Standby))
    }

    // Check SleepingTimeout
    if config.DiskControlled.SleepingTimeout < 0 ||
            config.DiskControlled.SleepingTimeout > 3600 {
        return config, errors.New(fmt.Sprintf("Invalid sleeping_timeout: %d not in [0, 3600]",
            config.DiskControlled.SleepingTimeout))
    }

    // Check TargetTemperature
    if config.DiskControlled.TargetTemperature < 0 ||
            config.DiskControlled.TargetTemperature > 100 {
        return config, errors.New(fmt.Sprintf("Invalid target_temp: %d not in [0, 100]",
            config.DiskControlled.TargetTemperature))
    }

    return config, nil
}

// https://en.wikipedia.org/wiki/PID_controller#Pseudocode
type PID struct {
    SetPoint float64

    KP float64
    KI float64
    KD float64

    previousError float64
    integral float64
    previousTime time.Time
}

func (pid *PID) Reset() {
    pid.previousError = 0
    pid.integral = 0
    pid.previousTime = time.Now()
}

func (pid *PID) UpdatePID(value float64) (float64) {
    timeSince := time.Since(pid.previousTime).Seconds()

    error := pid.SetPoint - value
    pid.integral = pid.integral + (error * timeSince)
    derivative := (error - pid.previousError) / (timeSince)
    pid.previousError = error

    return (pid.KP * error) + (pid.KI * pid.integral) + (pid.KD * derivative)
}

// Apply configuration and run indefinitely
func monitorTemperature(config Config) {
    diskGroup := DiskGroup{config.DiskControlled.Disks}
    controller := FanController{DevicePath: config.DevicePath}

    constantSet := false

    // Default is asleep in case of service restart. This means that if the
    // cooldown did not finish, then the cooldown will be shortened, but we
    // want that to avoid fan spinup on service restart.
    lastStatus := DISK_STATUS_ASLEEP
    deadlineOff := time.Now()
    lastRPM := -1

    // Use PID control
    // FIXME: tune parameters
    pid := PID{
        SetPoint: float64(config.DiskControlled.TargetTemperature),
        KP: 3, KI: 5, KD: 3,
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

            case DISK_STATUS_ASLEEP:
                // Disks are turned off - turn off fans after a cooldown period
                if lastStatus == DISK_STATUS_ASLEEP {
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

            case DISK_STATUS_STANDBY:
                // Disks are neither fully turned off, and neither active
                // Can't read temperature in this state
                targetRPM = config.DiskControlled.RPM.Standby
                log.Printf("INFO Disk status is standby, setting RPM to: %d", targetRPM)

            case DISK_STATUS_ACTIVE:
                // Disks are active - check temperature
                if lastStatus != DISK_STATUS_ACTIVE {
                    pid.Reset()
                } else {
                    pid.UpdatePID(float64(temp))
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
    if  !((len(os.Args) == 3 && os.Args[2] == "daemon") ||
            (len(os.Args) == 4 && os.Args[2] == "get") ||
            (len(os.Args) == 5 && os.Args[2] == "set")) {
        fmt.Fprintf(os.Stderr, "Usage: %v\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  JSON_CONFIG_FILE daemon\n")
        fmt.Fprintf(os.Stderr, "  JSON_CONFIG_FILE get all|1|2|3|4|5|6\n")
        fmt.Fprintf(os.Stderr, "  JSON_CONFIG_FILE set all|1|2|3|4|5|6 0|20|21|...|100\n")
        return
    }

    config, err := readConfig(os.Args[1])

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
            if value, err := strconv.Atoi(os.Args[3]); err != nil || !isValidFan(value) {
                fmt.Fprintf(os.Stderr, "Bad fan index: %v\n", os.Args[3])
                return
            } else {
                fans = append(fans, value)
            }
        }

        // Parse rpm
        rpm := 0
        if os.Args[2] == "set" {
            if value, err := strconv.Atoi(os.Args[4]); err != nil || !isValidRPM(value) {
                fmt.Fprintf(os.Stderr, "Bad fan RPM: %v\n", os.Args[4])
                return
            } else {
                rpm = value
            }
        }

        // Open controller
        controller := FanController{DevicePath: config.DevicePath}
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
                if rpm, err := controller.GetSpeed(fan); err != nil {
                    fmt.Fprintf(os.Stderr, "Failed to get speed: %v\n", err)
                    return
                } else {
                    fmt.Printf("%d %d\n", fan, rpm)
                }
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
