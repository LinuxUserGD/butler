package which

import (
	"os"

	"github.com/LinuxUserGD/butler/buildinfo"
	"github.com/LinuxUserGD/butler/comm"
	"github.com/LinuxUserGD/butler/mansion"
)

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("which", "Prints the path to this binary")
	ctx.Register(cmd, do)
}

func do(ctx *mansion.Context) {
	p, err := os.Executable()
	ctx.Must(err)

	comm.Logf("You're running butler %s, from the following path:", buildinfo.VersionString)
	comm.Logf("%s", p)
}
