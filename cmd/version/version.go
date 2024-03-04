package version

import (
	"log"
	"time"

	"github.com/LinuxUserGD/butler/buildinfo"
	"github.com/LinuxUserGD/butler/comm"
	"github.com/LinuxUserGD/butler/mansion"
)

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("version", "Prints the current version of butler")
	ctx.Register(cmd, do)
}

type VersionData struct {
	Version       string     `json:"version"`
	BuiltAt       *time.Time `json:"builtAt"`
	Commit        string     `json:"commit"`
	VersionString string     `json:"versionString"`
}

func do(ctx *mansion.Context) {
	if ctx.JSON {
		comm.Result(VersionData{
			Version:       buildinfo.Version,
			BuiltAt:       buildinfo.BuildTime(),
			Commit:        buildinfo.Commit,
			VersionString: buildinfo.VersionString,
		})
	} else {
		log.Println(buildinfo.VersionString)
	}
}
