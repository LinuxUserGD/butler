package sign

import (
	"context"
	"os"
	"time"

	"github.com/LinuxUserGD/butler/comm"
	"github.com/LinuxUserGD/butler/filtering"
	"github.com/LinuxUserGD/butler/mansion"

	"github.com/itchio/lake/pools"
	"github.com/itchio/lake/tlc"

	"github.com/itchio/wharf/pwr"
	"github.com/itchio/wharf/wire"
	"github.com/itchio/wharf/wsync"

	"github.com/itchio/headway/united"

	"github.com/pkg/errors"
)

var args = struct {
	output    *string
	signature *string
	fixPerms  *bool
}{}

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("sign", "(Advanced) Generate a signature file for a given directory. Useful for integrity checks and remote diff generation.")
	args.output = cmd.Arg("dir", "Path of directory to sign").Required().String()
	args.signature = cmd.Arg("signature", "Path to write signature to").Required().String()
	args.fixPerms = cmd.Flag("fix-permissions", "Detect Mac & Linux executables and adjust their permissions automatically").Default("true").Bool()
	ctx.Register(cmd, do)
}

func do(ctx *mansion.Context) {
	ctx.Must(Do(*args.output, *args.signature, ctx.CompressionSettings(), *args.fixPerms))
}

func Do(output string, signature string, compression pwr.CompressionSettings, fixPerms bool) error {
	comm.Opf("Creating signature for %s", output)
	startTime := time.Now()

	container, err := tlc.WalkAny(output, tlc.WalkOpts{Filter: filtering.FilterPaths})
	if err != nil {
		return errors.Wrap(err, "walking directory to sign")
	}

	pool, err := pools.New(container, output)
	if err != nil {
		return errors.Wrap(err, "creating pool for directory to sign")
	}

	if fixPerms {
		container.FixPermissions(pool)
	}

	signatureWriter, err := os.Create(signature)
	if err != nil {
		return errors.Wrap(err, "creating signature file")
	}
	defer signatureWriter.Close()

	rawSigWire := wire.NewWriteContext(signatureWriter)
	rawSigWire.WriteMagic(pwr.SignatureMagic)

	rawSigWire.WriteMessage(&pwr.SignatureHeader{
		Compression: &compression,
	})

	sigWire, err := pwr.CompressWire(rawSigWire, &compression)
	if err != nil {
		return errors.Wrap(err, "setting up compression for signature file")
	}
	sigWire.WriteMessage(container)

	comm.StartProgress()
	err = pwr.ComputeSignatureToWriter(context.Background(), container, pool, comm.NewStateConsumer(), func(hash wsync.BlockHash) error {
		return sigWire.WriteMessage(&pwr.BlockHash{
			WeakHash:   hash.WeakHash,
			StrongHash: hash.StrongHash,
		})
	})
	comm.EndProgress()
	if err != nil {
		return errors.Wrap(err, "computing signature")
	}

	err = sigWire.Close()
	if err != nil {
		return errors.Wrap(err, "finalizing signature file")
	}

	prettySize := united.FormatBytes(container.Size)
	perSecond := united.FormatBPS(container.Size, time.Since(startTime))
	comm.Statf("%s (%s) @ %s/s\n", prettySize, container.Stats(), perSecond)

	return nil
}
