package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/itchio/ox"

	"github.com/LinuxUserGD/butler/manager"

	"github.com/itchio/dash"

	"github.com/mitchellh/mapstructure"

	"github.com/BurntSushi/toml"

	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/comm"
	"github.com/LinuxUserGD/butler/endpoints/launch"
	"github.com/LinuxUserGD/butler/mansion"
	"github.com/LinuxUserGD/butler/redist"
	"github.com/LinuxUserGD/hush/manifest"

	"github.com/itchio/httpkit/eos"
	"github.com/itchio/httpkit/eos/option"

	"github.com/itchio/headway/state"
	"github.com/itchio/headway/united"

	"github.com/pkg/errors"
)

var args = struct {
	dir      *string
	platform *string
	arch     *string
}{}

func Register(ctx *mansion.Context) {
	cmd := ctx.App.Command("validate", "Validate a build folder, including its maniest if any")
	args.dir = cmd.Arg("dir", "Path of build folder to validate").Required().String()
	args.platform = cmd.Flag("platform", "Platform to validate for").Enum(string(ox.PlatformLinux), string(ox.PlatformOSX), string(ox.PlatformWindows))
	args.arch = cmd.Flag("arch", "Architecture to validate for").Enum(string(dash.Arch386), string(dash.ArchAmd64))
	ctx.Register(cmd, doValidate)
}

func doValidate(ctx *mansion.Context) {
	ctx.Must(Validate(comm.NewStateConsumer()))
}

func Validate(consumer *state.Consumer) error {
	banner := func(banner string, msg string, args ...interface{}) {
		consumer.Infof("")
		consumer.Infof("================== %s ==================", banner)
		consumer.Infof(msg, args...)
		consumer.Infof("=============================================")
		consumer.Infof("")
	}

	showWarning := func(msg string, args ...interface{}) {
		banner("Warning", msg, args...)
	}

	errorCount := 0
	showError := func(msg string, args ...interface{}) {
		errorCount++
		banner("Error", msg, args...)
	}

	hasDir := false
	dir := *args.dir

	var manifestPath string
	dirStats, err := os.Stat(dir)
	if err != nil {
		return errors.Wrapf(err, "stat'ing %s", dir)
	}

	consumer.Infof("")
	if dirStats.IsDir() {
		comm.Opf("Validating build directory %s", dir)
		manifestPath = manifest.Path(dir)
		hasDir = true
	} else {
		comm.Opf("Validating manifest only")
		manifestPath = dir
	}

	runtime := ox.CurrentRuntime()
	if *args.platform != "" {
		runtime.Platform = ox.Platform(*args.platform)
	}
	if *args.arch != "" {
		runtime.Is64 = (*args.arch == string(dash.ArchAmd64))
	}
	host := manager.Host{Runtime: runtime}

	consumer.Infof("For host %v (use --platform and --arch to simulate others)", host)
	consumer.Infof("")

	if !hasDir {
		showWarning("In manifest-only validation mode. Pass a valid build directory to perform further checks.")
	}

	printStrategyResult := func(sr *butlerd.StrategyResult) {
		for _, line := range strings.Split(sr.String(), "\n") {
			consumer.Infof("    %s", line)
		}
	}

	showHeuristics := func() error {
		consumer.Infof("")
		consumer.Infof("Heuristics will be used to launch your project.")
		if hasDir {
			verdict, err := manager.Configure(consumer, dir, runtime)
			if err != nil {
				return errors.Wrapf(err, "automatically determing launch targets for %s", dir)
			}

			consumer.Infof("")
			consumer.Statf("Heuristic results (best first):")

			for i, candidate := range verdict.Candidates {
				consumer.Infof("")
				consumer.Infof("  → Implicit launch target %d", i+1)
				target, err := launch.CandidateToLaunchTarget(nil, dir, host, candidate)
				if err != nil {
					showError(err.Error())
				} else {
					printStrategyResult(target.Strategy)
				}
			}
		} else {
			showWarning("Pass a complete build folder to see launch heuristic results")
		}
		return nil
	}

	stats, err := os.Stat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			consumer.Infof("No manifest found (expected it to be at %s)", manifestPath)
			err := showHeuristics()
			if err != nil {
				return errors.Wrap(err, "showing heuristics")
			}
			return nil
		}
		return errors.Wrap(err, "stat'ing manifest file")
	}

	consumer.Opf("Validating %s manifest at (%s)", united.FormatBytes(stats.Size()), manifestPath)

	var intermediate map[string]interface{}
	_, err = toml.DecodeFile(manifestPath, &intermediate)
	if err != nil {
		consumer.Errorf("Parse error:")
		return errors.Wrap(err, "parsing manifest")
	}

	jsonIntermediate, err := json.MarshalIndent(intermediate, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshalling manifest as json")
	}
	consumer.Debugf("Intermediate:\n%s", string(jsonIntermediate))

	appManifest := &manifest.Manifest{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      appManifest,
		ErrorUnused: true,
	})
	if err != nil {
		consumer.Errorf("Internal error:")
		return errors.Wrap(err, "decoding manifest from json form")
	}

	err = decoder.Decode(intermediate)
	if err != nil {
		warnOnly := false
		if mse, ok := err.(*mapstructure.Error); ok {
			warnOnly = true
			for _, e := range mse.Errors {
				if strings.Contains(e, "has invalid keys") {
					// cool!
				} else {
					warnOnly = false
					break
				}
			}
		}

		if warnOnly {
			showWarning("%s", err.Error())
		} else {
			consumer.Errorf("Decoding error:")
			return errors.Wrap(err, "decoding manifest")
		}
	}

	_, err = toml.DecodeFile(manifestPath, appManifest)
	if err != nil {
		return errors.Wrap(err, "parsing toml manifest")
	}

	jsonManifest, err := json.MarshalIndent(appManifest, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshalling manifest as json")
	}

	consumer.Debugf("Manifest:\n%s", string(jsonManifest))

	consumer.Infof("")
	if len(appManifest.Actions) > 0 {
		consumer.Statf("Validating %d actions...", len(appManifest.Actions))
		for _, action := range appManifest.Actions {
			consumer.Infof("")
			consumer.Infof("  → Action '%s' (%s)", action.Name, action.Path)
			if action.Platform != "" {
				switch action.Platform {
				case ox.PlatformLinux:
					consumer.Infof("    Only for Linux")
				case ox.PlatformOSX:
					consumer.Infof("    Only for macOS")
				case ox.PlatformWindows:
					consumer.Infof("    Only for Windows")
				default:
					showError("Unknown platform specified: (%s)", action.Platform)
				}
			}
			if action.Scope != "" {
				consumer.Infof("    Requests API scope (%s)", action.Scope)
			}
			if action.Sandbox {
				consumer.Infof("    Sandbox opt-in")
			}
			if action.Console {
				consumer.Infof("    Console")
			}
			if len(action.Args) > 0 {
				consumer.Infof("    Passes arguments: %s", strings.Join(action.Args, " ::: "))
			}
			if hasDir {
				target, err := launch.ActionToLaunchTarget(consumer, host, dir, action)
				if err != nil {
					showError(err.Error())
				} else {
					printStrategyResult(target.Strategy)
				}
			}
		}
	} else {
		consumer.Statf("No actions found.")
		err := showHeuristics()
		if err != nil {
			return errors.Wrap(err, "showing heuristics")
		}
	}

	consumer.Infof("")
	if len(appManifest.Prereqs) > 0 {
		consumer.Statf("Validating %d prereqs...", len(appManifest.Prereqs))
		consumer.Infof("")

		regFile, err := eos.Open("https://broth.itch.zone/itch-redists/info/LATEST/unpacked", option.WithConsumer(consumer))
		if err != nil {
			return errors.Wrap(err, "opening prereqs registry")
		}

		reg := &redist.RedistRegistry{}
		err = json.NewDecoder(regFile).Decode(reg)
		if err != nil {
			return errors.Wrap(err, "decoding prereqs registry")
		}

		for _, p := range appManifest.Prereqs {
			entry := reg.Entries[p.Name]
			if entry == nil {
				showError("Unknown prerequisite listed: %s", p.Name)
				continue
			}
			consumer.Infof("  → %s (%s)", entry.FullName, p.Name)
			var platforms []string
			if entry.Windows != nil {
				platforms = append(platforms, "Windows")
			}
			if entry.Linux != nil {
				platforms = append(platforms, "Linux")
			}
			if entry.OSX != nil {
				platforms = append(platforms, "macOS")
			}
			consumer.Infof("    Available on %s for architecture %s", strings.Join(platforms, ", "), entry.Arch)
			consumer.Infof("")
		}
	} else {
		consumer.Statf("No prereqs listed.")
		consumer.Infof("")
		consumer.Infof("If your application needs some libraries to pre-installed (.NET, Visual C++ Runtime, etc.),")
		consumer.Infof("you can list them in the manifest.")
		consumer.Infof("")
		consumer.Infof("Visit https://itch.io/docs/itch/integrating/manifest.html for more information.")
	}

	if errorCount > 0 {
		return fmt.Errorf("Found %d errors.", errorCount)
	}

	return nil
}
