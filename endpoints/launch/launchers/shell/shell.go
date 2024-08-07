package shell

import (
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/butlerd/messages"
	"github.com/LinuxUserGD/butler/endpoints/launch"
	"github.com/pkg/errors"
)

func Register() {
	launch.RegisterLauncher(butlerd.LaunchStrategyShell, &Launcher{})
}

type Launcher struct{}

var _ launch.Launcher = (*Launcher)(nil)

func (l *Launcher) Do(params launch.LauncherParams) error {
	_, err := messages.ShellLaunch.Call(params.RequestContext, butlerd.ShellLaunchParams{
		ItemPath: params.FullTargetPath,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
