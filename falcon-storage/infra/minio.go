package infra

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
)

var (
	minioOnce         sync.Once
	minioClient       *minio.Client
	minioPublicClient *minio.Client
	minioURL          string
)

// GetMinio returns the process-wide MinIO client, initialising it on first call.
func GetMinio() *minio.Client {
	minioOnce.Do(initMinio)
	return minioClient
}

// GetMinioPublic returns a MinIO client configured with the public endpoint.
// Use this for PresignedPutObject so the returned URL is device-accessible.
func GetMinioPublic() *minio.Client {
	minioOnce.Do(initMinio)
	return minioPublicClient
}

func initMinio() {
	values, err := helpers.ReadEnvs("MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_PUBLIC_URL")
	if err != nil {
		logrus.Fatalf("infra/minio: %v", err)
	}
	endpoint, accessKey, secretKey, publicURL := values[0], values[1], values[2], values[3]
	minioURL = publicURL

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		logrus.Fatalf("infra/minio: new client: %v", err)
	}
	minioClient = mc

	// Parse MINIO_PUBLIC_URL (e.g. "http://192.168.1.10:9000") into host:port for the public client.
	publicEndpoint, secure := parseMinioEndpoint(publicURL)
	mcp, err := minio.New(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		logrus.Fatalf("infra/minio: new public client: %v", err)
	}
	minioPublicClient = mcp
}

// MinioPublicURL returns the public base URL for MinIO (e.g. http://localhost:9000).
func MinioPublicURL() string {
	GetMinio() // ensure initialised
	return minioURL
}

// parseMinioEndpoint strips the scheme from a URL like "http://host:9000"
// and returns ("host:9000", false) or ("host:9000", true) for https.
func parseMinioEndpoint(rawURL string) (endpoint string, secure bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// rawURL might already be host:port — use as-is
		return strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://"), strings.HasPrefix(rawURL, "https://")
	}
	return u.Host, u.Scheme == "https"
}

// EnsureBucket creates bucket if it does not exist. Pass publicRead=true to set
// a public read-only bucket policy (suitable for logos, avatars, etc.).
func EnsureBucket(ctx context.Context, bucket string, publicRead bool) error {
	mc := GetMinio()

	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q: %w", bucket, err)
	}
	if exists {
		return nil
	}

	if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("make bucket %q: %w", bucket, err)
	}
	logrus.Infof("minio: created bucket %q", bucket)

	if publicRead {
		policy := fmt.Sprintf(
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`,
			bucket,
		)
		if err := mc.SetBucketPolicy(ctx, bucket, policy); err != nil {
			logrus.Warnf("minio: set public read policy on %q: %v", bucket, err)
		}
	}

	return nil
}
