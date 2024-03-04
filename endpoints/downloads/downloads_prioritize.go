package downloads

import (
	"crawshaw.io/sqlite"
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/database/models"
)

func DownloadsPrioritize(rc *butlerd.RequestContext, params butlerd.DownloadsPrioritizeParams) (*butlerd.DownloadsPrioritizeResult, error) {
	var download *models.Download
	rc.WithConn(func(conn *sqlite.Conn) {
		download = ValidateDownload(conn, params.DownloadID)
		download.Position = models.DownloadMinPosition(conn) - 1
		download.Save(conn)
	})

	res := &butlerd.DownloadsPrioritizeResult{}
	return res, nil
}
