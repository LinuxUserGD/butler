package install

import (
	"github.com/go-errors/errors"
	"github.com/itchio/butler/buse"
	"github.com/itchio/butler/database/models"
	uuid "github.com/satori/go.uuid"
)

func InstallLocationsGetByID(rc *buse.RequestContext, params *buse.InstallLocationsGetByIDParams) (*buse.InstallLocationsGetByIDResult, error) {
	if params.ID == "" {
		return nil, errors.Errorf("id must be set")
	}

	il := models.InstallLocationByID(rc.DB(), params.ID)
	if il == nil {
		return nil, errors.Errorf("install location (%s) not found", params.ID)
	}

	// TODO: fill disk space etc.

	res := &buse.InstallLocationsGetByIDResult{
		InstallLocation: &buse.InstallLocationSummary{
			ID:   il.ID,
			Path: il.Path,
			SizeInfo: &buse.InstallLocationSizeInfo{
				// TODO: fill in
				InstalledSize: -1,
				FreeSize:      -1,
				TotalSize:     -1,
			},
		},
	}
	return res, nil
}

func InstallLocationsList(rc *buse.RequestContext, params *buse.InstallLocationsListParams) (*buse.InstallLocationsListResult, error) {
	var locations []*models.InstallLocation
	err := rc.DB().Find(&locations).Error
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var flocs []*buse.InstallLocationSummary
	for _, il := range locations {
		flocs = append(flocs, &buse.InstallLocationSummary{
			ID:   il.ID,
			Path: il.Path,
			SizeInfo: &buse.InstallLocationSizeInfo{
				// TODO: fill in
				InstalledSize: -1,
				FreeSize:      -1,
				TotalSize:     -1,
			},
		})
	}

	res := &buse.InstallLocationsListResult{
		InstallLocations: flocs,
	}
	return res, nil
}

func InstallLocationsAdd(rc *buse.RequestContext, params *buse.InstallLocationsAddParams) (*buse.InstallLocationsAddResult, error) {
	consumer := rc.Consumer

	hadID := false
	if params.ID == "" {
		hadID = true
		freshUuid, err := uuid.NewV4()
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		params.ID = freshUuid.String()
	}
	if params.Path == "" {
		return nil, errors.New("path must be set")
	}

	if hadID {
		existing := models.InstallLocationByID(rc.DB(), params.ID)
		if existing != nil {
			if existing.Path == params.Path {
				consumer.Statf("(%s) exists, and has same path (%s), doing nothing", params.ID, params.Path)
				res := &buse.InstallLocationsAddResult{}
				return res, nil
			}
			return nil, errors.Errorf("(%s) exists but has path (%s) - we were passed (%s)", params.ID, existing.Path, params.Path)
		}
	}

	il := &models.InstallLocation{
		ID:   params.ID,
		Path: params.Path,
	}
	err := rc.DB().Save(il).Error
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := &buse.InstallLocationsAddResult{}
	return res, nil
}

func InstallLocationsRemove(rc *buse.RequestContext, params *buse.InstallLocationsRemoveParams) (*buse.InstallLocationsRemoveResult, error) {
	consumer := rc.Consumer

	if params.ID == "" {
		return nil, errors.Errorf("id must be set")
	}

	il := models.InstallLocationByID(rc.DB(), params.ID)
	if il == nil {
		consumer.Statf("Install location (%s) does not exist, doing nothing")
		res := &buse.InstallLocationsRemoveResult{}
		return res, nil
	}

	var caveCount int64
	err := rc.DB().Model(&models.Cave{}).Where("install_location_id = ?", il.ID).Count(&caveCount).Error
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if caveCount > 0 {
		// TODO: suggest moving to another install location
		return nil, errors.Errorf("Refusing to remove install location (%s) because it is not empty", params.ID)
	}

	var locationCount int64
	err = rc.DB().Model(&models.InstallLocation{}).Count(&locationCount).Error
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if locationCount == 1 {
		return nil, errors.Errorf("Refusing to remove last install location")
	}

	err = rc.DB().Delete(il).Error
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := &buse.InstallLocationsRemoveResult{}
	return res, nil
}