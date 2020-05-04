
Overview
========

Command line and daemon NZXT Grid+ v2 Digital Fan Controller

Based on [CapitalF/gridfan](https://github.com/CapitalF/gridfan)

Usage
=====

Compile

```bash
go build
```

Modify *sample.yaml*

Interactive CLI: immediately get/set values. Does not use any locking on
the serial device, so be careful if using this in any automated scripts.

```bash
./gridfan sample.yaml get all
./gridfan sample.yaml get 1
./gridfan sample.yaml get 6
./gridfan sample.yaml set all 50
./gridfan sample.yaml set 3 20
```

Daemon: gridfan in the foreground forever. Sets *constant_rpm* fans once on
startup. Sets *curve_fans* fans depending on temperature and status of
disks (active, standby, sleeping). Requires *hddtemp* and *hdparm* commands
to be installed.

```bash
./gridfan daemon sample.yaml
```

Disk Curve Pseudocode:

```python
while True:
    temp, status = disks.poll()
    if status == sleeping:
        if time_since_sleep >= cooldown_timeout:
            fans.set(rpm.sleeping)
        else:
            fans.set(rpm.cooldown)
    elif status == standby:
        fans.set(rpm.standby)
    else:
        rpm = 0
        for point in points:
            if temp >= point.temp:
                rpm = point.temp
        fans.set(rpm)

    sleep(poll_interval)
```
