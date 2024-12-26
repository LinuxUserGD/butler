package push

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"time"

	"github.com/itchio/httpkit/eos"
	"github.com/itchio/httpkit/eos/option"
	"github.com/itchio/httpkit/uploader"

	itchio "github.com/itchio/go-itchio"

	"github.com/itchio/butler/comm"
	"github.com/itchio/butler/filtering"
	"github.com/itchio/butler/mansion"

	"github.com/itchio/headway/counter"
	"github.com/itchio/headway/state"
	"github.com/itchio/headway/united"

	"github.com/itchio/savior/seeksource"

	"github.com/itchio/lake"
	"github.com/itchio/lake/tlc"

	"github.com/itchio/wharf/pwr"
	"github.com/itchio/wharf/wsync"
	"github.com/pkg/errors"
)

const (
	// almostThereThreshold is the amount of data left where the progress indicator isn't indicative anymore.
	// At this point, we're basically waiting for build files to be finalized.
	almostThereThreshold int64 = 10 * 1024
)

// pushing single files with any of these extensions
// will show a warning
var singleFileWarningExtensionList = []string{
	".exe", // probably an NSIS installer or something
	".msi", // Microsoft package
	".pkg", // Apple installer
}

var args = struct {
	src             string
	target          string
	userVersion     string
	userVersionFile string
	fixPerms        bool
	dereference     bool
	ifChanged       bool
	dryRun          bool
	autoWrap        bool
}{}

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("push", "Upload a new build to itch.io. See `butler help push`.")
	cmd.Arg("src", "Directory to upload. May also be a zip archive (slower)").Required().StringVar(&args.src)
	cmd.Arg("target", "Where to push, for example 'leafo/x-moon:win-64'. Targets are of the form project:channel, where project is username/game or game_id.").Required().StringVar(&args.target)
	cmd.Flag("userversion", "A user-supplied version number that you can later query builds by").StringVar(&args.userVersion)
	cmd.Flag("userversion-file", "A file containing a user-supplied version number that you can later query builds by").StringVar(&args.userVersionFile)
	cmd.Flag("fix-permissions", "Detect Mac & Linux executables and adjust their permissions automatically").Default("true").BoolVar(&args.fixPerms)
	cmd.Flag("dereference", "Dereference symlinks").Default("false").BoolVar(&args.dereference)
	cmd.Flag("if-changed", "Don't push anything if it would be an empty patch").Default("false").BoolVar(&args.ifChanged)
	cmd.Flag("dry-run", "Don't push anything, just show what would be pushed").Default("false").BoolVar(&args.dryRun)
	cmd.Flag("auto-wrap", "Apply workaround for https://github.com/itchio/itch/issues/2147").Default("true").BoolVar(&args.autoWrap)
	ctx.Register(cmd, do)
}

func do(ctx *mansion.Context) {
	go ctx.DoVersionCheck()

	// if userVersionFile specified, read from the given file
	// TODO: do utf-16 decoding here
	userVersion := args.userVersion
	if userVersion == "" && args.userVersionFile != "" {
		buf, err := ioutil.ReadFile(args.userVersionFile)
		ctx.Must(err)

		userVersion = strings.TrimSpace(string(buf))
		if strings.ContainsAny(userVersion, "\r\n") {
			ctx.Must(fmt.Errorf("%s contains line breaks, refusing to use as userversion", args.userVersionFile))
		}
	}

	ctx.Must(Do(ctx, args.src, args.target, userVersion, args.fixPerms, args.dereference, args.ifChanged, args.autoWrap))
}

func Do(ctx *mansion.Context, buildPath string, specStr string, userVersion string, fixPerms bool, dereference bool, ifChanged bool, wrap bool) error {
	consumer := comm.NewStateConsumer()

	// start walking source container while waiting on auth flow
	sourceContainerChan := make(chan walkResult)
	walkErrs := make(chan error)
	walkOpts := tlc.WalkOpts{
		Filter:      filtering.FilterPaths,
		Dereference: dereference,
	}
	if wrap {
		walkOpts.AutoWrap(&buildPath, consumer)
	}

	go doWalk(buildPath, sourceContainerChan, walkErrs, fixPerms, walkOpts)

	if args.dryRun {
		comm.Opf("Dry run, listing files we would push...")
		select {
		case walkErr := <-walkErrs:
			return errors.Wrap(walkErr, "walking directory to push")
		case walkies := <-sourceContainerChan:
			log := func(line string) {
				comm.Logf(line)
			}
			walkies.container.Print(log)
			comm.Statf("Would push %s", walkies.container)
		}
		return nil
	}

	spec, err := itchio.ParseSpec(specStr)
	if err != nil {
		return errors.Wrapf(err, "parsing push target '%s'", specStr)
	}

	err = spec.EnsureChannel()
	if err != nil {
		return err
	}

	client, err := ctx.AuthenticateViaOauth()
	if err != nil {
		return errors.Wrap(err, "authenticating")
	}

	getSignature := func(ID int64) (*pwr.SignatureInfo, error) {
		buildFiles, err := client.ListBuildFiles(ctx.DefaultCtx(), ID)
		if err != nil {
			return nil, errors.Wrap(err, "listing build files")
		}

		signatureFile := itchio.FindBuildFile(itchio.BuildFileTypeSignature, buildFiles.Files)
		if signatureFile == nil {
			comm.Dief("Could not find signature for parent build %d, aborting", ID)
		}

		signatureURL := client.MakeBuildFileDownloadURL(itchio.MakeBuildFileDownloadURLParams{
			BuildID: ID,
			FileID:  signatureFile.ID,
		})

		signatureReader, err := eos.Open(signatureURL, option.WithConsumer(consumer))
		if err != nil {
			return nil, errors.Wrap(err, "opening signature")
		}
		defer signatureReader.Close()

		signatureSource := seeksource.FromFile(signatureReader)

		_, err = signatureSource.Resume(nil)
		if err != nil {
			return nil, errors.Wrap(err, "opening signature")
		}

		signature, err := pwr.ReadSignature(context.Background(), signatureSource)
		if err != nil {
			return nil, errors.Wrap(err, "reading signature")
		}

		return signature, nil
	}

	if ifChanged {
		chanInfo, err := client.GetChannel(ctx.DefaultCtx(), spec.Target, spec.Channel)
		if err == nil && chanInfo != nil && chanInfo.Channel != nil && chanInfo.Channel.Head != nil {
			comm.Opf("Comparing against previous build...")
			sig, err := getSignature(chanInfo.Channel.Head.ID)
			if err != nil {
				return errors.Wrap(err, "getting previous build signature")
			}

			err = pwr.AssertValid(buildPath, sig)
			if err == nil {
				comm.Statf("No changes and --if-changed used, not pushing anything")
				return nil
			}

			if _, ok := err.(*pwr.ErrHasWound); ok {
				// cool, that's what we expected
			} else {
				return errors.Wrap(err, "checking for differences")
			}
		} else {
			comm.Opf("No previous build to compare against, pushing unconditionally")
		}
	}

	newBuildRes, err := client.CreateBuild(ctx.DefaultCtx(), itchio.CreateBuildParams{
		Target:      spec.Target,
		Channel:     spec.Channel,
		UserVersion: userVersion,
	})
	if err != nil {
		return errors.Wrap(err, "creating build on remote server")
	}

	buildID := newBuildRes.Build.ID
	parentID := newBuildRes.Build.ParentBuild.ID

	var targetSignature *pwr.SignatureInfo

	if parentID == 0 {
		comm.Opf("For channel `%s`: pushing first build", spec.Channel)
		targetSignature = &pwr.SignatureInfo{
			Container: &tlc.Container{},
			Hashes:    make([]wsync.BlockHash, 0),
		}
	} else {
		comm.Opf("For channel `%s`: last build is %d, downloading its signature", spec.Channel, parentID)
		var err error
		targetSignature, err = getSignature(parentID)
		if err != nil {
			return errors.Wrap(err, "searching for parent build signature")
		}
	}

	bothFiles, err := createBothFiles(ctx, client, buildID)
	if err != nil {
		return errors.Wrap(err, "creating remote patch and signature files")
	}

	newPatchRes := bothFiles.patchRes
	newSignatureRes := bothFiles.signatureRes

	patchWriter := uploader.NewResumableUpload(newPatchRes.File.UploadURL)
	patchWriter.SetConsumer(consumer)

	signatureWriter := uploader.NewResumableUpload(newSignatureRes.File.UploadURL)
	signatureWriter.SetConsumer(consumer)

	comm.Debugf("Launching patch & signature channels")

	patchCounter := counter.NewWriter(patchWriter)
	signatureCounter := counter.NewWriter(signatureWriter)

	// we started walking the source container in the beginning,
	// we actually need it now.
	// note that we could actually start diffing before all the file
	// creation & upload setup is done

	var sourceContainer *tlc.Container
	var sourcePool lake.Pool

	comm.Debugf("Waiting for source container")
	select {
	case walkErr := <-walkErrs:
		return errors.Wrap(walkErr, "walking directory to push")
	case walkies := <-sourceContainerChan:
		sourceContainer = walkies.container
		sourcePool = walkies.pool
		break
	}

	showSingleFileWarningIfNecessary(sourceContainer)

	err = sourceContainer.Validate()
	if err != nil {
		comm.Notice("Validation failed", []string{
			fmt.Sprintf("(%s) cannot be pushed, because it is invalid.", buildPath),
			"",
			"If you're pushing a .zip file, try pushing a folder directly instead. Pushing a folder is not only faster, it eliminates a whole class of errors.",
			"",
			"The errors found duration validation follow.",
		})
		comm.Logf("%s", err)
		comm.Die("Refusing to push invalid container (see above)")
	}

	comm.Opf("Pushing %s", sourceContainer)

	comm.Debugf("Building diff context")
	var readBytes int64

	var bytesPerSec float64
	var lastUploadedBytes int64
	var patchUploadedBytes int64

	stopTicking := make(chan struct{})
	updateProgress := func() {
		// input bytes that aren't in output, for example:
		//  - bytes that have been compressed away
		//  - bytes that were in old build and were simply reused
		goneBytes := readBytes - patchCounter.Count()

		conservativeTotalBytes := sourceContainer.Size - goneBytes

		leftBytes := conservativeTotalBytes - patchUploadedBytes
		if leftBytes > almostThereThreshold {
			netStatus := "- network idle"
			if bytesPerSec > 1 {
				netStatus = fmt.Sprintf("@ %s/s", united.FormatBytes(int64(bytesPerSec)))
			}
			comm.ProgressLabel(fmt.Sprintf("%s, %s left", netStatus, united.FormatBytes(leftBytes)))
		} else {
			comm.ProgressLabel(fmt.Sprintf("- almost there"))
		}

		conservativeProgress := float64(patchUploadedBytes) / float64(conservativeTotalBytes)
		conservativeProgress = min(1.0, conservativeProgress)
		comm.Progress(conservativeProgress)

		comm.ProgressScale(float64(readBytes) / float64(sourceContainer.Size))
	}

	patchWriter.SetProgressListener(func(count int64) {
		patchUploadedBytes = count
		updateProgress()
	})

	go func() {
		ticker := time.NewTicker(time.Second * time.Duration(2))
		for {
			select {
			case <-ticker.C:
				bytesPerSec = float64(patchUploadedBytes-lastUploadedBytes) / 2.0
				lastUploadedBytes = patchUploadedBytes
				updateProgress()
			case <-stopTicking:
				return
			}
		}
	}()

	stateConsumer := &state.Consumer{
		OnProgress: func(progress float64) {
			readBytes = int64(float64(sourceContainer.Size) * progress)
			updateProgress()
		},
	}

	dctx := &pwr.DiffContext{
		Compression: &pwr.CompressionSettings{
			Algorithm: pwr.CompressionAlgorithm_BROTLI,
			Quality:   1,
		},

		SourceContainer: sourceContainer,
		Pool:            sourcePool,

		TargetContainer: targetSignature.Container,
		TargetSignature: targetSignature.Hashes,

		Consumer: stateConsumer,
	}

	comm.StartProgress()
	comm.ProgressScale(0.0)
	err = dctx.WritePatch(context.Background(), patchCounter, signatureCounter)
	if err != nil {
		return errors.Wrap(err, "computing and writing patch")
	}

	// close both files concurrently
	{
		errs := make(chan error)

		go func() {
			errs <- patchWriter.Close()
		}()
		go func() {
			errs <- signatureWriter.Close()
		}()

		// 2 close
		for i := 0; i < 2; i++ {
			err := <-errs
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	close(stopTicking)
	comm.ProgressLabel("finalizing build")

	// finalize both files concurrently
	{
		errs := make(chan error)

		doFinalize := func(fileID int64, fileSize int64, done chan error) {
			_, err = client.FinalizeBuildFile(ctx.DefaultCtx(), itchio.FinalizeBuildFileParams{
				BuildID: buildID,
				FileID:  fileID,
				Size:    fileSize,
			})
			done <- err
		}

		go doFinalize(newPatchRes.File.ID, patchCounter.Count(), errs)
		go doFinalize(newSignatureRes.File.ID, signatureCounter.Count(), errs)

		// 2 doFinalize
		for i := 0; i < 2; i++ {
			err := <-errs
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	comm.EndProgress()

	{
		prettyPatchSize := united.FormatBytes(patchCounter.Count())
		percReused := 100.0 * float64(dctx.ReusedBytes) / float64(dctx.FreshBytes+dctx.ReusedBytes)
		relToNew := 100.0 * float64(patchCounter.Count()) / float64(sourceContainer.Size)
		prettyFreshSize := united.FormatBytes(dctx.FreshBytes)
		savings := 100.0 - relToNew

		if dctx.ReusedBytes > 0 {
			comm.Statf("Re-used %.2f%% of old, added %s fresh data", percReused, prettyFreshSize)
		} else {
			comm.Statf("Added %s fresh data", prettyFreshSize)
		}

		if savings > 0 && !math.IsNaN(savings) {
			comm.Statf("%s patch (%.2f%% savings)", prettyPatchSize, 100.0-relToNew)
		} else {
			comm.Statf("%s patch (no savings)", prettyPatchSize)
		}
	}
	comm.Opf("Build is now processing, should be up in a bit.")
	comm.Logf("")
	comm.Logf("Use the `butler status %s` for more information.", specStr)
	comm.Logf("")

	return nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func showSingleFileWarningIfNecessary(sourceContainer *tlc.Container) {
	if !sourceContainer.IsSingleFile() {
		return
	}

	singlePath := sourceContainer.Files[0].Path
	lowerPath := strings.ToLower(singlePath)

	deservesWarning := false
	for _, ext := range singleFileWarningExtensionList {
		if strings.HasSuffix(lowerPath, ext) {
			deservesWarning = true
			break
		}
	}

	if !deservesWarning {
		return
	}

	comm.Notice("You're pushing a single file", []string{
		"Diffing and patching work poorly on 'all-in-one executables' and installers. Consider pushing a portable build instead, for optimal distribution.",
		"",
		"For more information, see https://itch.io/docs/butler/single-files.html",
	})
}
