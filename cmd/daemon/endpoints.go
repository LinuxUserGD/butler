package daemon

import (
	"crawshaw.io/sqlite/sqlitex"
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/butlerd/messages"
	"github.com/LinuxUserGD/butler/endpoints/cleandownloads"
	"github.com/LinuxUserGD/butler/endpoints/downloads"
	"github.com/LinuxUserGD/butler/endpoints/fetch"
	"github.com/LinuxUserGD/butler/endpoints/install"
	"github.com/LinuxUserGD/butler/endpoints/launch"
	"github.com/LinuxUserGD/butler/endpoints/meta"
	"github.com/LinuxUserGD/butler/endpoints/profile"
	"github.com/LinuxUserGD/butler/endpoints/search"
	"github.com/LinuxUserGD/butler/endpoints/system"
	"github.com/LinuxUserGD/butler/endpoints/tests"
	"github.com/LinuxUserGD/butler/endpoints/update"
	"github.com/LinuxUserGD/butler/endpoints/utilities"
	"github.com/LinuxUserGD/butler/mansion"
)

var mainRouter *butlerd.Router

func GetRouter(dbPool *sqlitex.Pool, mansionContext *mansion.Context) *butlerd.Router {
	if mainRouter != nil {
		return mainRouter
	}

	mainRouter = butlerd.NewRouter(dbPool, mansionContext.NewClient, mansionContext.HTTPClient, mansionContext.HTTPTransport)

	meta.Register(mainRouter)
	utilities.Register(mainRouter)
	tests.Register(mainRouter)
	update.Register(mainRouter)
	install.Register(mainRouter)
	launch.Register(mainRouter)
	cleandownloads.Register(mainRouter)
	profile.Register(mainRouter)
	fetch.Register(mainRouter)
	downloads.Register(mainRouter)
	search.Register(mainRouter)
	system.Register(mainRouter)

	messages.EnsureAllRequests(mainRouter)

	return mainRouter
}
