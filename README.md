
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
startup. Sets *disk_controlled* fans depending on temperature and status of
disks (active, standby, sleeping), and required *hddtemp* and *hdparm* commands
to be installed.

```bash
./gridfan daemon
```
