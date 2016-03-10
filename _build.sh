#!/usr/bin/env bash

esc -o assets/assets.go -pkg assets static
gox -output="bin/{{.Dir}}_{{.OS}}_{{.Arch}}" -os="windows darwin linux freebsd openbsd netbsd"
