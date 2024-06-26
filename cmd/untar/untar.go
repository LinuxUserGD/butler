package untar

import (
	"github.com/LinuxUserGD/butler/comm"
	"github.com/LinuxUserGD/butler/mansion"
	"github.com/itchio/wharf/archiver"
	"github.com/pkg/errors"
)

var args = struct {
	file *string
	dir  *string
}{}

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("untar", "Extract a .tar file").Hidden()
	args.file = cmd.Arg("file", "Path of the .tar archive to extract").Required().String()
	args.dir = cmd.Flag("dir", "An optional directory to which to extract files (defaults to CWD)").Default(".").Short('d').String()
	ctx.Register(cmd, do)
}

func do(ctx *mansion.Context) {
	ctx.Must(Do(ctx, *args.file, *args.dir))
}

func Do(ctx *mansion.Context, file string, dir string) error {
	settings := archiver.ExtractSettings{
		Consumer: comm.NewStateConsumer(),
	}

	comm.StartProgress()
	res, err := archiver.ExtractTar(file, dir, settings)
	comm.EndProgress()

	if err != nil {
		return errors.WithStack(err)
	}
	comm.Logf("Extracted %d dirs, %d files, %d symlinks", res.Dirs, res.Files, res.Symlinks)

	return nil
}
