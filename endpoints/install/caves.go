package install

import (
	"crawshaw.io/sqlite"
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/database/models"
)

func CavesSetPinned(rc *butlerd.RequestContext, params butlerd.CavesSetPinnedParams) (*butlerd.CavesSetPinnedResult, error) {
	rc.WithConn(func(conn *sqlite.Conn) {
		cave := models.CaveByID(conn, params.CaveID)
		cave.Pinned = params.Pinned
		cave.Save(conn)
	})

	return &butlerd.CavesSetPinnedResult{}, nil
}
