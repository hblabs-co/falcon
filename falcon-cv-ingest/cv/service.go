package cv

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/qdrant/qdrant"
)

const presignedURLExpiry = 15 * time.Minute

// PrepareResult is returned to the client after /cv/prepare.
type PrepareResult struct {
	CVID      string    `json:"cv_id"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Service orchestrates the CV ingest pipeline.
type Service struct {
	minio       *minio.Client
	minioBucket string
	embeddings  *embeddingsClient
	qdrant      *qdrant.Client
}

// NewService initialises all clients and ensures the MinIO bucket and Qdrant
// collection exist. Fatals on misconfiguration.
func NewService() (*Service, error) {
	values, err := helpers.ReadEnvs("MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET")
	if err != nil {
		return nil, err
	}

	endpoint, accessKey, secretKey, bucket := values[0], values[1], values[2], values[3]
	useSSL := helpers.ReadEnvOptional("MINIO_USE_SSL", "false") == "true"

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx := system.Ctx()

	// Ensure bucket exists.
	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
		logrus.Infof("minio: created bucket %q", bucket)
	}

	emb, err := newEmbeddingsClient()
	if err != nil {
		return nil, fmt.Errorf("embeddings client: %w", err)
	}

	qdr, err := qdrant.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}
	if err := qdr.EnsureCollection(ctx); err != nil {
		return nil, fmt.Errorf("qdrant ensure collection: %w", err)
	}

	srv := &Service{
		minio:       mc,
		minioBucket: bucket,
		embeddings:  emb,
		qdrant:      qdr,
	}

	return srv, nil
}

// Prepare creates a pending CV record in MongoDB and returns a presigned
// MinIO PUT URL that the client uses to upload the file directly.
// No user identity is required at this stage.
func (s *Service) Prepare(ctx context.Context, filename string) (*PrepareResult, error) {
	id := gonanoid.Must()
	key := fmt.Sprintf("cvs/%s/%s", id, filename)
	expiresAt := time.Now().Add(presignedURLExpiry)

	uploadURL, err := s.minio.PresignedPutObject(ctx, s.minioBucket, key, presignedURLExpiry)
	if err != nil {
		return nil, fmt.Errorf("generate presigned URL: %w", err)
	}

	cv := &models.PersistedCV{
		ID:          id,
		Filename:    filename,
		MinioBucket: s.minioBucket,
		MinioKey:    key,
		Status:      models.CVStatusPendingUpload,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := system.GetStorage().SetById(ctx, constants.MongoCVsCollection, id, cv); err != nil {
		return nil, fmt.Errorf("save cv record: %w", err)
	}

	return &PrepareResult{
		CVID:      id,
		UploadURL: uploadURL.String(),
		ExpiresAt: expiresAt,
	}, nil
}

// Index triggers async processing of a CV that has already been uploaded to MinIO.
// email is used to upsert a User record and obtain the user_id that is carried
// through the entire pipeline.
// It returns immediately; the caller polls or waits for the cv.indexed event.
func (s *Service) Index(cvID, email string) error {
	ctx := context.Background()

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, cvID, &cv); err != nil {
		return fmt.Errorf("cv not found: %w", err)
	}
	if cv.Status == models.CVStatusIndexed || cv.Status == models.CVStatusIndexing {
		return fmt.Errorf("cv already %s", cv.Status)
	}

	userID, err := s.upsertUser(ctx, email)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	cv.UserID = userID
	s.setStatus(ctx, cvID, models.CVStatusIndexing, "")

	go s.process(cv)
	return nil
}

// upsertUser returns the existing user ID for email, or creates a new User and
// returns its ID.
func (s *Service) upsertUser(ctx context.Context, email string) (string, error) {
	var user models.User
	err := system.GetStorage().Get(ctx, constants.MongoUsersCollection, bson.M{"email": email}, &user)
	if err == nil {
		return user.ID, nil
	}

	now := time.Now()
	user = models.User{
		ID:        gonanoid.Must(),
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := system.GetStorage().SetById(ctx, constants.MongoUsersCollection, user.ID, &user); err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}

// process runs the full pipeline: download → extract → embed → qdrant → event.
// Runs in a background goroutine; updates MongoDB status on success or failure.
func (s *Service) process(cv models.PersistedCV) {
	ctx := context.Background()
	log := logrus.WithField("cv_id", cv.ID)

	fail := func(msg string, err error) {
		log.Errorf("%s: %v", msg, err)
		s.setStatus(ctx, cv.ID, models.CVStatusFailed, fmt.Sprintf("%s: %v", msg, err))
	}

	// 1. Download from MinIO.
	obj, err := s.minio.GetObject(ctx, cv.MinioBucket, cv.MinioKey, minio.GetObjectOptions{})
	if err != nil {
		fail("download from minio", err)
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		fail("stat minio object", err)
		return
	}

	// 2. Extract text from .docx.
	text, err := extractText(obj, info.Size)
	if err != nil {
		fail("extract text", err)
		return
	}
	if text == "" {
		fail("extract text", fmt.Errorf("document appears to be empty"))
		return
	}
	log.Infof("extracted %d chars from %s", len(text), cv.Filename)

	// 3. Generate embedding.
	vector, err := s.embeddings.Embed(ctx, text)
	if err != nil {
		fail("generate embedding", err)
		return
	}
	log.Infof("embedding generated (%d dims)", len(vector))

	// 4. Upsert into Qdrant.
	// Qdrant point IDs must be a UUID or uint64 — use a dedicated UUID, not the nanoid CV ID.
	qdrantID := uuid.New().String()
	payload := map[string]string{"cv_id": cv.ID, "user_id": cv.UserID, "filename": cv.Filename}
	err = s.qdrant.Upsert(ctx, qdrantID, vector, payload)
	if err != nil {
		fail("qdrant upsert", err)
		return
	}

	// 5. Update MongoDB.
	now := time.Now()
	if err := system.GetStorage().Set(ctx, constants.MongoCVsCollection, bson.M{"id": cv.ID}, bson.M{
		"status":     models.CVStatusIndexed,
		"qdrant_id":  qdrantID,
		"error":      "",
		"updated_at": now,
	}); err != nil {
		log.Errorf("update cv status to indexed: %v", err)
	}

	// 6. Publish cv.indexed event.
	event := models.CVIndexedEvent{
		CVID:     cv.ID,
		UserID:   cv.UserID,
		QdrantID: qdrantID,
	}
	if err := system.Publish(ctx, constants.SubjectCVIndexed, event); err != nil {
		log.Errorf("publish cv.indexed: %v", err)
	} else {
		log.Infof("published cv.indexed")
	}
}

// Run starts the HTTP server. It blocks until the server stops.
func (s *Service) Run() error {
	port := helpers.ReadEnvOptional("PORT", "8081")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	r.SetTrustedProxies(nil)

	routes(r, s)
	return r.Run(":" + port)
}

// GetCV fetches a CV record from MongoDB by ID.
func (s *Service) GetCV(ctx context.Context, cvID string) (*models.PersistedCV, error) {
	var cv models.PersistedCV
	err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, cvID, &cv)
	if err != nil {
		return nil, err
	}
	return &cv, nil
}

func (s *Service) setStatus(ctx context.Context, cvID string, status models.CVStatus, errMsg string) {
	doc := bson.M{"status": status, "error": errMsg, "updated_at": time.Now()}
	_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cvID, doc)
}
