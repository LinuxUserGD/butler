package fetch

import (
	"crawshaw.io/sqlite"
	"xorm.io/builder"
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/database/models"
)

func FetchExpireAll(rc *butlerd.RequestContext, params butlerd.FetchExpireAllParams) (*butlerd.FetchExpireAllResult, error) {
	rc.WithConn(func(conn *sqlite.Conn) {
		models.MustDelete(conn, &models.FetchInfo{}, builder.Expr("1"))
	})
	res := &butlerd.FetchExpireAllResult{}
	return res, nil
}
