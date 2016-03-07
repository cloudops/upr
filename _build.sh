#!/usr/bin/env bash

gox -output="bin/{{.Dir}}_{{.OS}}_{{.Arch}}" -os="windows darwin linux freebsd openbsd netbsd"