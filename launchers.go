package main

import (
	"github.com/LinuxUserGD/butler/endpoints/launch/launchers/html"
	"github.com/LinuxUserGD/butler/endpoints/launch/launchers/native"
	"github.com/LinuxUserGD/butler/endpoints/launch/launchers/shell"
	"github.com/LinuxUserGD/butler/endpoints/launch/launchers/url"
)

func init() {
	native.Register()
	shell.Register()
	html.Register()
	url.Register()
}
