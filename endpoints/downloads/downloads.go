package downloads

import (
	"github.com/LinuxUserGD/butler/butlerd"
	"github.com/LinuxUserGD/butler/butlerd/messages"
)

func Register(router *butlerd.Router) {
	messages.DownloadsQueue.Register(router, DownloadsQueue)
	messages.DownloadsPrioritize.Register(router, DownloadsPrioritize)
	messages.DownloadsList.Register(router, DownloadsList)
	messages.DownloadsDrive.Register(router, DownloadsDrive)
	messages.DownloadsDriveCancel.Register(router, DownloadsDriveCancel)
	messages.DownloadsClearFinished.Register(router, DownloadsClearFinished)
	messages.DownloadsDiscard.Register(router, DownloadsDiscard)
	messages.DownloadsRetry.Register(router, DownloadsRetry)
}
