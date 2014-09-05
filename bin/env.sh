#!/bin/bash

export GOPATH=~/go
export PATH=~/go/bin:$PATH

# Godep dependency manager
if [[ $(which godep) == "" ]]; then
    echo "Installing godep."
    go get github.com/tools/godep
fi

# Run go oracle for development https://godoc.org/code.google.com/p/go.tools/oracle
if [[ $(which oracle) == "" ]]; then
    echo "Setting up go oracle for source code analysis."
    go install code.google.com/p/go.tools/cmd/oracle
fi

if [[ $(which godoc) == "" ]]; then
    echo "Godoc not installed."
fi
