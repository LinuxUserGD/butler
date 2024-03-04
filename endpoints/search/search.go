package search

import (
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/butlerd/messages"
)

func Register(router *butlerd.Router) {
	messages.SearchGames.Register(router, SearchGames)
	messages.SearchUsers.Register(router, SearchUsers)
}
