package prereqs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchio/butler/redist"
	"github.com/itchio/ox"

	"github.com/itchio/headway/state"

	"github.com/itchio/butler/cmd/msi"
	"github.com/itchio/butler/shell"
	"github.com/itchio/butler/mansion"
	"github.com/pkg/errors"
)

func Install(ctx *mansion.Context, planPath string, pipePath string) error {
	planReader, err := os.Open(planPath)
	if err != nil {
		return errors.WithStack(err)
	}

	decoder := json.NewDecoder(planReader)

	plan := &PrereqPlan{}
	err = decoder.Decode(plan)
	if err != nil {
		return errors.WithStack(err)
	}

	namedPipe, err := NewNamedPipe(pipePath)
	if err != nil {
		return errors.WithStack(err)
	}

	consumer := namedPipe.Consumer()

	consumer.Infof("Installing %d prereqs", len(plan.Tasks))
	startTime := time.Now()

	var failed []string

	runtime := ox.CurrentRuntime()

	for _, task := range plan.Tasks {
		taskStartTime := time.Now()
		namedPipe.WriteState(task.Name, "installing")

		consumer.Infof("")
		consumer.Infof("# Installing %s", task.Name)
		consumer.Infof("")

		var err error
		switch runtime.Platform {
		case ox.PlatformWindows:
			err = installWindowsPrereq(consumer, task)
		case ox.PlatformLinux:
			err = installLinuxPrereq(consumer, task)
		default:
			return fmt.Errorf("Don't know how to install prereqs for platform %s", runtime.Platform)
		}

		if err != nil {
			consumer.Errorf("For prereq (%s): %+v", task.Name, err)
			failed = append(failed, task.Name)
		}

		namedPipe.WriteState(task.Name, "done")
		consumer.Infof("(Spent %s)", time.Since(taskStartTime))
	}

	consumer.Infof("")
	if len(failed) > 0 {
		errMsg := fmt.Sprintf("Some prereqs failed to install: %s", strings.Join(failed, ", "))
		consumer.Errorf(errMsg)
		return errors.New(errMsg)
	}

	consumer.Statf("All done! (Spent %s total)", time.Since(startTime))

	return nil
}

func installWindowsPrereq(consumer *state.Consumer, task *PrereqTask) error {
	block := task.Info.Windows
	if block == nil {
		return errors.Errorf("No windows block for prereq %s", task.Name)
	}

	commandPath := filepath.Join(task.WorkDir, block.Command)
	args := block.Args

	// MSI packages get special treatment for reasons.
	if strings.HasSuffix(strings.ToLower(block.Command), ".msi") {
		tempDir, err := ioutil.TempDir("", "butler-msi-logs")
		if err != nil {
			return errors.WithStack(err)
		}

		defer func() {
			os.RemoveAll(tempDir)
		}()

		logPath := filepath.Join(tempDir, "msi-install-log.txt")

		err = msi.Install(consumer, commandPath, logPath, nil)
		if err != nil {
			return fmt.Errorf("MSI install failed: %s", err.Error())
		}
	} else {
		cmdTokens := append([]string{commandPath}, args...)
		signedCode, err := shell.RunCommand(consumer, cmdTokens)
		if err != nil {
			return errors.WithStack(err)
		}

		if signedCode != 0 {
			code := uint32(signedCode)
			for _, exitCode := range block.ExitCodes {
				if code == exitCode.Code {
					if exitCode.Success {
						consumer.Infof("%s (Code %d), continuing", exitCode.Message, exitCode.Code)
						return nil
					} else {
						return fmt.Errorf("%s (Code %d), we'll error out eventually", exitCode.Message, exitCode.Code)
					}
				}
			}

			return fmt.Errorf("Got unknown exit code 0x%X (%d), will error out", code, code)
		}
	}

	return nil
}

func installLinuxPrereq(consumer *state.Consumer, task *PrereqTask) error {
	block := task.Info.Linux

	switch block.Type {
	case redist.LinuxRedistTypeHosted:
		for _, f := range block.EnsureExecutable {
			path := filepath.Join(task.WorkDir, f)
			consumer.Infof("Making (%s) executable", path)
			err := os.Chmod(path, 0o755)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		for _, f := range block.EnsureSuidRoot {
			path := filepath.Join(task.WorkDir, f)
			consumer.Infof("Making (%s) SUID", path)

			err := os.Chown(path, 0, 0)
			if err != nil {
				return errors.WithStack(err)
			}

			err = os.Chmod(path, 0o755|os.ModeSetuid)
			if err != nil {
				return errors.WithStack(err)
			}
		}
	default:
		return fmt.Errorf("Don't know how to install linux redist type (%s)", block.Type)
	}

	return nil
}
