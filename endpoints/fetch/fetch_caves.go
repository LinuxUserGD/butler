package fetch

import (
	"crawshaw.io/sqlite"
	"github.com/itchio/butler/butlerd"
	"github.com/itchio/butler/database/models"
	"github.com/itchio/butler/endpoints/fetch/pager"
	"github.com/itchio/hades"
	"xorm.io/builder"
)

func FetchCaves(rc *butlerd.RequestContext, params butlerd.FetchCavesParams) (*butlerd.FetchCavesResult, error) {
	res := &butlerd.FetchCavesResult{}

	rc.WithConn(func(conn *sqlite.Conn) {
		var cond = builder.NewCond()
		joinGames := false
		search := hades.Search{}

		switch params.SortBy {
		case "title":
			ordering := pager.Ordering("ASC", params.Reverse)
			search = search.OrderBy("lower(games.title) " + ordering)
			joinGames = true
		case "playTime":
			ordering := pager.Ordering("DESC", params.Reverse)
			search = search.OrderBy("caves.seconds_run " + ordering)
		case "installedAt":
			ordering := pager.Ordering("DESC", params.Reverse)
			search = search.OrderBy("caves.installed_at " + ordering)
		case "installedSize":
			ordering := pager.Ordering("DESC", params.Reverse)
			search = search.OrderBy("caves.installed_size " + ordering)
		case "lastTouched", "":
			ordering := pager.Ordering("DESC", params.Reverse)
			search = search.OrderBy("coalesce(caves.last_touched_at, caves.installed_at) " + ordering)
		}

		if params.Filters.Classification != "" {
			cond = builder.And(cond, builder.Eq{"games.classification": params.Filters.Classification})
			joinGames = true
		}

		if params.Filters.InstallLocationID != "" {
			cond = builder.And(cond, builder.Eq{"caves.install_location_id": params.Filters.InstallLocationID})
		}

		if params.Filters.GameID != 0 {
			cond = builder.And(cond, builder.Eq{"caves.game_id": params.Filters.GameID})
		}

		if params.Search != "" {
			cond = builder.And(cond, builder.Like{"games.title", params.Search})
			joinGames = true
		}

		if joinGames {
			search = search.InnerJoin("games", "games.id = caves.game_id")
		}

		var items []*models.Cave
		pg := pager.New(params)
		res.NextCursor = pg.Fetch(conn, &items, cond, search)
		models.PreloadCaves(conn, items)
		for _, cave := range items {
			res.Items = append(res.Items, FormatCave(conn, cave))
		}
	})
	return res, nil
}
