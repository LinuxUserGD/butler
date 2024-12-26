package native

import (
	"fmt"
	"strings"

	"github.com/itchio/butler/butlerd/messages"
	"github.com/itchio/ox"

	"github.com/itchio/butler/butlerd"
	"github.com/itchio/butler/cmd/prereqs"
	"github.com/itchio/butler/endpoints/launch"
	"github.com/pkg/errors"
)

func handlePrereqs(params launch.LauncherParams) error {
	ph, err := prereqs.NewHandler(prereqs.Params{
		RequestContext: params.RequestContext,
		APIKey:         params.Access.APIKey,
		Host:           params.Host,
		Consumer:       params.RequestContext.Consumer,
		PrereqsDir:     params.PrereqsDir,
		Force:          params.ForcePrereqs,
	})
	if err != nil {
		return err
	}

	err = handleUE4Prereqs(params)
	if err != nil {
		return errors.WithMessage(err, "While handling UE4 prereqs")
	}

	consumer := params.RequestContext.Consumer

	var wanted []string

	// add manifest prereqs
	if params.AppManifest == nil {
		consumer.Infof("No manifest, no prereqs")
		autoPrereqs, err := handleAutoPrereqs(params, ph)
		if err != nil {
			return errors.WithMessage(err, "While doing auto prereqs")
		}

		wanted = append(wanted, autoPrereqs...)
	} else {
		if len(params.AppManifest.Prereqs) == 0 {
			consumer.Infof("Got manifest but no prereqs requested")
		} else {
			for _, p := range params.AppManifest.Prereqs {
				wanted = append(wanted, p.Name)
			}
		}
	}

	// append built-in params if we need some
	{
		runtime := params.Host.Runtime
		if runtime.Platform == ox.PlatformLinux && params.Sandbox {
			firejailName := fmt.Sprintf("firejail-%s", runtime.Arch())
			wanted = append(wanted, firejailName)
		}
	}

	if len(wanted) == 0 {
		return nil
	}

	if params.PrereqsDir == "" {
		return errors.New("PrereqsDir cannot be empty")
	}

	var pending []string
	for _, name := range wanted {
		if ph.HasInstallMarker(name) {
			continue
		}

		pending = append(pending, name)
	}

	pending, err = ph.FilterPrereqs(pending)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(pending) == 0 {
		consumer.Infof("✓ %d Prereqs already installed or irrelevant: %s", len(wanted), strings.Join(wanted, ", "))
		return nil
	}

	pa, err := ph.AssessPrereqs(pending)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(pa.Done) > 0 {
		consumer.Infof("✓ %d Prereqs already done: %s", len(pa.Done), strings.Join(pa.Done, ", "))
	}

	if len(pa.Todo) == 0 {
		consumer.Infof("Everything done!")
		return nil
	}
	consumer.Infof("→ %d Prereqs to install: %s", len(pa.Todo), strings.Join(pa.Todo, ", "))

	{
		psn := butlerd.PrereqsStartedNotification{
			Tasks: make(map[string]*butlerd.PrereqTask),
		}
		for i, name := range pa.Todo {
			entry, err := ph.GetEntry(name)
			if err != nil {
				return errors.WithStack(err)
			}

			psn.Tasks[name] = &butlerd.PrereqTask{
				FullName: entry.FullName,
				Order:    i,
			}
		}

		err = messages.PrereqsStarted.Notify(params.RequestContext, psn)
		if err != nil {
			consumer.Warnf(err.Error())
		}
	}

	tsc := &prereqs.TaskStateConsumer{
		OnState: func(state butlerd.PrereqsTaskStateNotification) {
			err = messages.PrereqsTaskState.Notify(params.RequestContext, state)
			if err != nil {
				consumer.Warnf(err.Error())
			}
		},
	}

	err = ph.FetchPrereqs(tsc, pa.Todo)
	if err != nil {
		return errors.WithStack(err)
	}

	plan, err := ph.BuildPlan(pa.Todo)
	if err != nil {
		return errors.WithStack(err)
	}

	err = ph.InstallPrereqs(tsc, plan)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, name := range pa.Todo {
		err = ph.MarkInstalled(name)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	err = messages.PrereqsEnded.Notify(params.RequestContext, butlerd.PrereqsEndedNotification{})
	if err != nil {
		consumer.Warnf(err.Error())
	}

	return nil
}
