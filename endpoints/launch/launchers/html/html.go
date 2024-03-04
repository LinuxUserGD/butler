package html

import (
	"path/filepath"

	"github.com/LinuxUserGD/butler/butlerd/messages"

	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/endpoints/launch"
	"github.com/pkg/errors"
)

func Register() {
	launch.RegisterLauncher(butlerd.LaunchStrategyHTML, &Launcher{})
}

type Launcher struct{}

var _ launch.Launcher = (*Launcher)(nil)

func (l *Launcher) Do(params launch.LauncherParams) error {
	rootFolder := params.InstallFolder
	indexPath, err := filepath.Rel(rootFolder, params.FullTargetPath)
	if err != nil {
		return errors.WithStack(err)
	}

	messages.LaunchRunning.Notify(params.RequestContext, butlerd.LaunchRunningNotification{})
	params.SessionStarted()

	_, err = messages.HTMLLaunch.Call(params.RequestContext, butlerd.HTMLLaunchParams{
		RootFolder: rootFolder,
		IndexPath:  indexPath,
		Args:       params.Args,
		Env:        params.Env,
	})
	messages.LaunchExited.Notify(params.RequestContext, butlerd.LaunchExitedNotification{})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
