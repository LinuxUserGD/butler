package main

import (
	"github.com/LinuxUserGD/butler/cmd/apply"
	"github.com/LinuxUserGD/butler/cmd/auditzip"
	"github.com/LinuxUserGD/butler/cmd/clean"
	"github.com/LinuxUserGD/butler/cmd/configure"
	"github.com/LinuxUserGD/butler/cmd/cp"
	"github.com/LinuxUserGD/butler/cmd/daemon"
	"github.com/LinuxUserGD/butler/cmd/diag"
	"github.com/LinuxUserGD/butler/cmd/diff"
	"github.com/LinuxUserGD/butler/cmd/ditto"
	"github.com/LinuxUserGD/butler/cmd/dl"
	"github.com/LinuxUserGD/butler/cmd/elevate"
	"github.com/LinuxUserGD/butler/cmd/elfprops"
	"github.com/LinuxUserGD/butler/cmd/exeprops"
	"github.com/LinuxUserGD/butler/cmd/extract"
	"github.com/LinuxUserGD/butler/cmd/fetch"
	"github.com/LinuxUserGD/butler/cmd/file"
	"github.com/LinuxUserGD/butler/cmd/fujicmd"
	"github.com/LinuxUserGD/butler/cmd/heal"
	"github.com/LinuxUserGD/butler/cmd/login"
	"github.com/LinuxUserGD/butler/cmd/logout"
	"github.com/LinuxUserGD/butler/cmd/ls"
	"github.com/LinuxUserGD/butler/cmd/mkdir"
	"github.com/LinuxUserGD/butler/cmd/mkzip"
	"github.com/LinuxUserGD/butler/cmd/msi"
	"github.com/LinuxUserGD/butler/cmd/pipe"
	"github.com/LinuxUserGD/butler/cmd/prereqs"
	"github.com/LinuxUserGD/butler/cmd/probe"
	"github.com/LinuxUserGD/butler/cmd/push"
	"github.com/LinuxUserGD/butler/cmd/ratetest"
	"github.com/LinuxUserGD/butler/cmd/rediff"
	"github.com/LinuxUserGD/butler/cmd/repack"
	"github.com/LinuxUserGD/butler/cmd/run"
	"github.com/LinuxUserGD/butler/cmd/sign"
	"github.com/LinuxUserGD/butler/cmd/singlediff"
	"github.com/LinuxUserGD/butler/cmd/sizeof"
	"github.com/LinuxUserGD/butler/cmd/status"
	"github.com/LinuxUserGD/butler/cmd/unsz"
	"github.com/LinuxUserGD/butler/cmd/untar"
	"github.com/LinuxUserGD/butler/cmd/unzip"
	"github.com/LinuxUserGD/butler/cmd/upgrade"
	"github.com/LinuxUserGD/butler/cmd/validate"
	"github.com/LinuxUserGD/butler/cmd/verify"
	"github.com/LinuxUserGD/butler/cmd/version"
	"github.com/LinuxUserGD/butler/cmd/walk"
	"github.com/LinuxUserGD/butler/cmd/which"
	"github.com/LinuxUserGD/butler/cmd/wipe"
	"github.com/LinuxUserGD/butler/mansion"
)

// Each of these specify their own arguments and flags in
// their own package.
func registerCommands(ctx *mansion.Context) {
	// documented commands

	login.Register(ctx)
	logout.Register(ctx)

	push.Register(ctx)
	fetch.Register(ctx)
	status.Register(ctx)

	file.Register(ctx)
	ls.Register(ctx)

	which.Register(ctx)
	version.Register(ctx)
	upgrade.Register(ctx)

	sign.Register(ctx)
	verify.Register(ctx)
	diff.Register(ctx)
	apply.Register(ctx)
	heal.Register(ctx)

	// hidden commands

	dl.Register(ctx)
	cp.Register(ctx)
	wipe.Register(ctx)
	sizeof.Register(ctx)
	mkdir.Register(ctx)
	ditto.Register(ctx)
	probe.Register(ctx)

	clean.Register(ctx)
	walk.Register(ctx)

	prereqs.Register(ctx)
	msi.Register(ctx)

	extract.Register(ctx)
	unzip.Register(ctx)
	unsz.Register(ctx)
	untar.Register(ctx)
	auditzip.Register(ctx)

	repack.Register(ctx)

	pipe.Register(ctx)
	elevate.Register(ctx)
	run.Register(ctx)

	exeprops.Register(ctx)
	elfprops.Register(ctx)

	configure.Register(ctx)

	daemon.Register(ctx)

	fujicmd.Register(ctx)
	validate.Register(ctx)

	singlediff.Register(ctx)
	rediff.Register(ctx)
	mkzip.Register(ctx)

	ratetest.Register(ctx)
	diag.Register(ctx)
}
