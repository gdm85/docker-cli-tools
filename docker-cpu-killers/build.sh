#!/bin/bash

export GOPATH="$HOME/goroot"

go get github.com/gdm85/go-libshell github.com/gdm85/go-dockerclient github.com/gdm85/goopt && \
go build
