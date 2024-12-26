package operate

import (
	"fmt"
	"strings"

	"crawshaw.io/sqlite"
	"xorm.io/builder"

	"github.com/itchio/butler/butlerd"
	"github.com/itchio/butler/database/models"
	"github.com/itchio/butler/manager"

	"github.com/itchio/hades"
	"github.com/itchio/headway/state"
	"github.com/itchio/headway/united"

	itchio "github.com/itchio/go-itchio"
	"github.com/pkg/errors"
)

func GameToString(game *itchio.Game) string {
	if game == nil {
		return "<nil game>"
	}

	return fmt.Sprintf("%s - %s", game.Title, game.URL)
}

func GetFilteredUploads(rc *butlerd.RequestContext, game *itchio.Game) (*manager.NarrowDownUploadsResult, error) {
	consumer := rc.Consumer

	var access *GameAccess
	rc.WithConn(func(conn *sqlite.Conn) {
		access = AccessForGameID(conn, game.ID)
	})
	client := rc.Client(access.APIKey)

	uploads, err := client.ListGameUploads(rc.Ctx, itchio.ListGameUploadsParams{
		GameID:      game.ID,
		Credentials: access.Credentials,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	consumer.Debugf("API returned %d uploads", len(uploads.Uploads))

	numInputs := len(uploads.Uploads)
	if numInputs == 0 {
		consumer.Infof("No uploads found at all (that we can access)")
	}
	uploadsFilterResult, err := manager.NarrowDownUploads(consumer, game, uploads.Uploads, rc.HostEnumerator())
	if err != nil {
		return nil, err
	}
	consumer.Debugf("Narrow returned %d uploads", len(uploadsFilterResult.Uploads))

	numResults := len(uploadsFilterResult.Uploads)

	if numInputs > 0 {
		if numResults == 0 {
			consumer.Infof("→ All uploads were filtered out")
		}
		qualif := fmt.Sprintf("these %d uploads", numResults)
		if numResults == 1 {
			qualif = "this upload"
		}

		consumer.Infof("→ Narrowed %d uploads down to %s: ", numInputs, qualif)
		for _, u := range uploadsFilterResult.Uploads {
			LogUpload(consumer, u, u.Build)
		}
	}

	return uploadsFilterResult, nil
}

func LogUpload(consumer *state.Consumer, u *itchio.Upload, b *itchio.Build) {
	if u == nil {
		consumer.Infof("  No upload")
	} else {
		var name string
		if u.DisplayName != "" {
			name = u.DisplayName
		} else {
			name = u.Filename
		}

		var size string
		if u.Size > 0 {
			size = united.FormatBytes(u.Size)
		} else {
			size = "Unknown size"
		}

		consumer.Infof("  ☁ %s :: %s :: #%d", name, size, u.ID)

		var plats []string
		if u.Platforms.Linux != "" {
			plats = append(plats, "Linux "+string(u.Platforms.Linux))
		}
		if u.Platforms.Windows != "" {
			plats = append(plats, "Windows "+string(u.Platforms.Windows))
		}
		if u.Platforms.OSX != "" {
			plats = append(plats, "macOS "+string(u.Platforms.OSX))
		}

		var platString = "No platforms"
		if len(plats) > 0 {
			platString = strings.Join(plats, ", ")
		}

		consumer.Infof("    %s :: %s", formatUploadType(u.Type), platString)
	}

	if b != nil {
		LogBuild(consumer, u, b)
	}
}

func LogBuild(consumer *state.Consumer, u *itchio.Upload, b *itchio.Build) {
	if b == nil {
		consumer.Infof("    Nil build")
	}

	version := ""
	if b.UserVersion != "" {
		version = b.UserVersion
	} else if b.Version != 0 {
		version = "No explicit version"
	}

	var bfs []string
	for _, bf := range b.Files {
		bfs = append(bfs, fmt.Sprintf("%s-%s", bf.Type, bf.SubType))
	}

	consumer.Infof("    Build %d for channel (%s) :: %s :: #%d", b.Version, u.ChannelName, version, b.ID)

	if len(bfs) > 0 {
		bfString := strings.Join(bfs, ", ")
		consumer.Infof("    %s", bfString)
	}
}

func formatUploadType(uploadType itchio.UploadType) string {
	switch uploadType {
	case itchio.UploadTypeDefault:
		return "Executable"

	case itchio.UploadTypeFlash:
		return "Flash object"
	case itchio.UploadTypeUnity:
		return "Legacy Unity Web"
	case itchio.UploadTypeJava:
		return "Java applet"

	case itchio.UploadTypeSoundtrack:
		return "Soundtrack"
	case itchio.UploadTypeBook:
		return "Book"
	case itchio.UploadTypeVideo:
		return "Video"
	case itchio.UploadTypeDocumentation:
		return "Documentation"
	case itchio.UploadTypeMod:
		return "Mod"
	case itchio.UploadTypeAudioAssets:
		return "Audio assets"
	case itchio.UploadTypeGraphicalAssets:
		return "Graphical assets"
	case itchio.UploadTypeSourcecode:
		return "Source code"

	case itchio.UploadTypeOther:
		return "Other"

	default:
		return fmt.Sprintf("(%s)", uploadType)
	}
}

type GameAccess struct {
	APIKey      string                 `json:"api_key"`
	Credentials itchio.GameCredentials `json:"credentials"`
}

func (ga *GameAccess) OnlyAPIKey() *GameAccess {
	return &GameAccess{
		APIKey: ga.APIKey,
	}
}

func AccessForGameID(conn *sqlite.Conn, gameID int64) *GameAccess {
	// TODO: write unit test for this

	// look for owner access
	{
		pgs := models.ProfileGamesByGameID(conn, gameID)
		if len(pgs) > 0 {
			pg := pgs[0]
			profile := models.ProfileByID(conn, pg.ProfileID)

			if profile != nil {
				access := &GameAccess{
					APIKey: profile.APIKey,
				}
				return access
			}
		}
	}

	// look for a download key
	{
		dks := models.DownloadKeysByGameID(conn, gameID)

		for _, dk := range dks {
			profile := models.ProfileByID(conn, dk.OwnerID)
			if profile == nil {
				continue
			}

			access := &GameAccess{
				APIKey: profile.APIKey,
				Credentials: itchio.GameCredentials{
					DownloadKeyID: dk.ID,
				},
			}
			return access
		}
	}

	// no special credentials
	{
		var profiles []*models.Profile
		models.MustSelect(conn, &profiles, builder.NewCond(), hades.Search{}.OrderBy("last_connected DESC"))
		if len(profiles) == 0 {
			panic(errors.New("No profiles found"))
		}

		// prefer press user
		for _, profile := range profiles {
			if profile.PressUser {
				access := &GameAccess{
					APIKey: profile.APIKey,
				}
				return access
			}
		}

		// just take the most recent then
		profile := profiles[0]
		access := &GameAccess{
			APIKey: profile.APIKey,
		}
		return access
	}
}

func ValidateCave(rc *butlerd.RequestContext, caveID string) *models.Cave {
	if caveID == "" {
		panic(errors.New("caveId must be set"))
	}

	var cave *models.Cave
	rc.WithConn(func(conn *sqlite.Conn) {
		cave = models.CaveByID(conn, caveID)
		if cave == nil {
			panic(fmt.Errorf("cave not found: (%s)", caveID))
		}
		cave.Preload(conn)
	})
	return cave
}

func FindBuildFile(files []*itchio.BuildFile, fileType itchio.BuildFileType, subType itchio.BuildFileSubType) *itchio.BuildFile {
	for _, f := range files {
		if f.Type == fileType && f.SubType == subType {
			if string(f.State) == "" {
				// previous API versions omitted build file state, use size instead
				if f.Size == 0 {
					// file is definitely not ready
					continue
				}
			} else {
				if f.State != itchio.BuildFileStateUploaded {
					// created/uploading files are listed but unusable
					continue
				}
			}
			return f
		}
	}
	return nil
}
