#!/bin/bash
set -euf -o pipefail


build() {
    GOOS="${1}"
    GOARCH="${2}"
    echo "Building ${GOOS} ${GOARCH}" 
    rm -f gridfan
    GOOS="${GOOS}" GOARCH="${GOARCH}" go build
    mv gridfan "gridfan.${GOOS}.${GOARCH}"
}

build darwin amd64
build linux amd64
