package company_logo

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
	"hblabs.co/falcon/common/system"
)

const companyLogosBucket = "company-logos"

// Service handles logo downloads and company metadata persistence.
type Service struct {
	minio          *minio.Client
	minioPublicURL string
}

// NewService initialises the MinIO client, ensures the bucket exists,
// and sets up the MongoDB index on companies.company_id.
func newService() (*Service, error) {
	values, err := helpers.ReadEnvs("MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_PUBLIC_URL")
	if err != nil {
		return nil, err
	}
	endpoint, accessKey, secretKey, publicURL := values[0], values[1], values[2], values[3]

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx := system.Ctx()

	exists, err := mc.BucketExists(ctx, companyLogosBucket)
	if err != nil {
		return nil, fmt.Errorf("minio check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, companyLogosBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
		policy := fmt.Sprintf(
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`,
			companyLogosBucket,
		)
		if err := mc.SetBucketPolicy(ctx, companyLogosBucket, policy); err != nil {
			logrus.Warnf("minio: set public read policy: %v", err)
		}
	}

	spec := system.CompoundIndexSpec{
		Collection: constants.MongoCompaniesCollection,
		Fields:     []string{"company_id"},
		Unique:     true,
	}
	if err := system.GetStorage().EnsureCompoundIndex(ctx, spec); err != nil {
		logrus.Warnf("ensure company_id index: %v", err)
	}

	return &Service{minio: mc, minioPublicURL: publicURL}, nil
}

// handleDownloadLogo processes a CompanyLogoDownloadRequestedEvent:
// skips if the company is already known, downloads the logo, stores it in
// MinIO, upserts the Company document in MongoDB, and publishes
// company.logo.downloaded.
func (s *Service) handleDownloadLogo(ctx context.Context, evt models.CompanyLogoDownloadRequestedEvent) error {
	log := logrus.WithFields(logrus.Fields{"company_id": evt.CompanyID, "source": evt.Source})

	var existing []models.Company
	if err := system.GetStorage().GetManyByField(
		ctx, constants.MongoCompaniesCollection, "company_id", []string{evt.CompanyID}, &existing,
	); err != nil {
		return fmt.Errorf("lookup company: %w", err)
	}

	if len(existing) > 0 {
		log.Debugf("company already synced, skipping")
		return nil
	}

	logoStorageURL := ""
	if evt.LogoURL != "" {
		data, contentType, err := ownhttp.DownloadBytes(evt.LogoURL)
		if err != nil {
			log.Warnf("download logo: %v", err)
		} else {
			ext := filepath.Ext(evt.LogoURL)
			if ext == "" {
				ext = ".jpg"
			}
			objectKey := fmt.Sprintf("%s/%s%s", evt.Source, evt.CompanyID, ext)
			_, err = s.minio.PutObject(
				ctx, companyLogosBucket, objectKey,
				bytes.NewReader(data), int64(len(data)),
				minio.PutObjectOptions{ContentType: contentType},
			)
			if err != nil {
				log.Warnf("upload logo to minio: %v", err)
			} else {
				logoStorageURL = fmt.Sprintf("%s/%s/%s", s.minioPublicURL, companyLogosBucket, objectKey)
			}
		}
	}

	company := models.NewCompany(evt.CompanyID, evt.CompanyName, logoStorageURL)
	if err := system.GetStorage().Set(
		ctx, constants.MongoCompaniesCollection,
		bson.M{"company_id": evt.CompanyID},
		company,
	); err != nil {
		return fmt.Errorf("upsert company: %w", err)
	}
	log.Infof("saved company %q logo=%s", evt.CompanyName, logoStorageURL)

	out := models.CompanyLogoDownloadedEvent{
		CompanyID:      evt.CompanyID,
		LogoStorageURL: logoStorageURL,
	}
	subject := constants.SubjectStorageCompanyLogoDownloaded
	if err := system.Publish(ctx, subject, out); err != nil {
		log.Warnf("publish %s: %v", subject, err)
	}

	return nil
}
