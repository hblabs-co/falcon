package company_logo

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/storage/infra"
)

const bucket = "company-logos"

type service struct{}

func newService(ctx context.Context) (*service, error) {
	if err := infra.EnsureBucket(ctx, bucket, true); err != nil {
		return nil, err
	}

	spec := system.CompoundIndexSpec{
		Collection: constants.MongoCompaniesCollection,
		Fields:     []string{"company_id"},
		Unique:     true,
	}
	if err := system.GetStorage().EnsureCompoundIndex(ctx, spec); err != nil {
		logrus.Warnf("company_logo: ensure index: %v", err)
	}

	return &service{}, nil
}

func (s *service) handle(ctx context.Context, evt models.CompanyLogoDownloadRequestedEvent, attempt system.RetryAttempt) error {
	log := logrus.WithFields(logrus.Fields{
		"company_id":   evt.CompanyID,
		"company_name": evt.CompanyName,
		"source":       evt.Source,
		"attempt":      attempt.Number,
	})
	log.Infof("received company logo request — logo_url=%s", evt.LogoURL)

	// Idempotency: skip if company already synced.
	var existing []models.Company
	if err := system.GetStorage().GetManyByField(
		ctx, constants.MongoCompaniesCollection, "company_id", []string{evt.CompanyID}, &existing,
	); err != nil {
		return fmt.Errorf("lookup company: %w", err)
	}
	if len(existing) > 0 {
		log.Infof("company already synced, skipping (logo=%s)", existing[0].LogoMinioURL)
		return nil
	}

	// No logo URL — save company without logo, no retry needed.
	if evt.LogoURL == "" {
		log.Warn("no logo URL provided, saving company without logo")
		return s.saveCompany(ctx, log, evt, "")
	}

	// Download logo.
	log.Infof("downloading logo from %s", evt.LogoURL)
	data, contentType, err := ownhttp.DownloadBytes(evt.LogoURL)
	if err != nil {
		if attempt.IsLast {
			s.saveServiceError(ctx, evt, constants.ErrNameLogoDownloadFailed, fmt.Errorf("download failed after all retries: %w", err))
			log.Errorf("logo download failed permanently, saving company without logo")
			return s.saveCompany(ctx, log, evt, "")
		}
		log.Warnf("logo download failed (will retry): %v", err)
		return fmt.Errorf("download logo: %w", err)
	}
	log.Infof("logo downloaded — size=%d bytes content_type=%s", len(data), contentType)

	// Upload to MinIO.
	ext := filepath.Ext(evt.LogoURL)
	if ext == "" {
		ext = ".jpg"
	}
	objectKey := fmt.Sprintf("%s/%s%s", evt.Source, evt.CompanyID, ext)
	_, err = infra.GetMinio().PutObject(
		ctx, bucket, objectKey,
		bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		if attempt.IsLast {
			s.saveServiceError(ctx, evt, constants.ErrNameLogoUploadFailed, fmt.Errorf("minio upload failed after all retries: %w", err))
			log.Errorf("logo upload failed permanently (key=%s), saving company without logo", objectKey)
			return s.saveCompany(ctx, log, evt, "")
		}
		log.Warnf("upload to minio failed (key=%s, will retry): %v", objectKey, err)
		return fmt.Errorf("upload logo: %w", err)
	}

	logoStorageURL := fmt.Sprintf("%s/%s/%s", infra.MinioPublicURL(), bucket, objectKey)
	log.Infof("logo uploaded to minio — url=%s", logoStorageURL)

	return s.saveCompany(ctx, log, evt, logoStorageURL)
}

func (s *service) saveCompany(ctx context.Context, log *logrus.Entry, evt models.CompanyLogoDownloadRequestedEvent, logoURL string) error {
	company := models.NewCompany(evt.CompanyID, evt.CompanyName, logoURL)
	if err := system.GetStorage().Set(
		ctx, constants.MongoCompaniesCollection,
		bson.M{"company_id": evt.CompanyID},
		company,
	); err != nil {
		return fmt.Errorf("upsert company: %w", err)
	}
	log.Infof("company saved — name=%q logo=%s", evt.CompanyName, logoURL)

	out := models.CompanyLogoDownloadedEvent{
		CompanyID:      evt.CompanyID,
		LogoStorageURL: logoURL,
	}
	if err := system.Publish(ctx, constants.SubjectStorageCompanyLogoDownloaded, out); err != nil {
		log.Warnf("publish %s: %v", constants.SubjectStorageCompanyLogoDownloaded, err)
	}
	return nil
}

func (s *service) saveServiceError(ctx context.Context, evt models.CompanyLogoDownloadRequestedEvent, errName string, cause error) {
	system.RecordError(ctx, models.ServiceError{
		ServiceName: constants.ServiceStorage,
		ErrorName:   errName,
		Error:       cause.Error(),
		Platform:    evt.Source,
		PlatformID:  evt.CompanyID,
		URL:         evt.LogoURL,
	})
}
