#!/bin/bash
export PATH="$PATH:/usr/local/go/bin"
export GOPATH=~/goroot

go get "github.com/gdm85/go-dockerclient" || exit $?

## build without debug information
go build -ldflags "-w -s"
