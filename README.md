
Overview
========

Command line and daemon NZXT Grid+ v2 Digital Fan Controller

Based on [CapitalF/gridfan](https://github.com/CapitalF/gridfan)

Usage
=====

Compile

```bash
go build -v cmd/gridfan
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

Disk Curve Pseudocode
=====================

```python
while True:
    # Check all disks, will *NOT* wake them up if power management policy has
    # put them to sleep.
    temp, status = disks.poll()

    if status == sleeping:
        if time_since_sleep >= cooldown_timeout:
            # Disks are sleeping, and we have spun fans at cooldown rpm for
            # cooldown timeout period, so now spin them at sleeping rpm.
            fans.set(rpm.sleeping)
        else:
            # Disks have just fallen asleep. Spin at cooldown rpm for cooldown
            # timeout period.
            fans.set(rpm.cooldown)

    elif status == standby:
        # Disks are in standby, and we can't get the temperature anymore.
        # Run fans at standby rpm.
        fans.set(rpm.standby)
    else:
        # Disks are awake. Find first marker that matches. Use 100 default in
        # case of no match.
        rpm = 100
        for point in points:
            if temp >= point.temp:
                rpm = point.temp
        fans.set(rpm)

    # Sleep for a little.
    sleep(poll_interval)
```
