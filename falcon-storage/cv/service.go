package cv

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/embeddings"
	"hblabs.co/falcon/modules/qdrant"
	"hblabs.co/falcon/storage/infra"
)

const (
	cvsBucket          = "cvs"
	presignedURLExpiry = 15 * time.Minute
)

type service struct {
	embeddings *embeddings.Client
	qdrant     *qdrant.Client
}

func newService(ctx context.Context) (*service, error) {
	if err := infra.EnsureBucket(ctx, cvsBucket, false); err != nil {
		return nil, err
	}

	if err := system.GetStorage().EnsureIndex(ctx,
		system.NewIndexSpec(constants.MongoCVsCollection, "user_id", false)); err != nil {
		return nil, fmt.Errorf("ensure cvs.user_id index: %w", err)
	}

	emb, err := embeddings.NewFromEnv()
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

	return &service{embeddings: emb, qdrant: qdr}, nil
}

// prepare creates a pending CV record and returns the presigned MinIO PUT URL
// as a CVPreparedEvent, which SubscribeCore sends back as the RPC reply.
func (s *service) prepare(ctx context.Context, evt models.CVPrepareRequestedEvent) (*models.CVPreparedEvent, error) {
	id := gonanoid.Must()
	key := fmt.Sprintf("cvs/%s/%s", id, evt.Filename)
	expiresAt := time.Now().Add(presignedURLExpiry)

	uploadURL, err := infra.GetMinioPublic().PresignedPutObject(ctx, cvsBucket, key, presignedURLExpiry)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}

	cv := &models.PersistedCV{
		ID:          id,
		Filename:    evt.Filename,
		MinioBucket: cvsBucket,
		MinioKey:    key,
		Status:      models.CVStatusPendingUpload,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := system.GetStorage().SetById(ctx, constants.MongoCVsCollection, id, cv); err != nil {
		return nil, fmt.Errorf("save cv record: %w", err)
	}

	logrus.Infof("[cv] prepared cv_id=%s request_id=%s", id, evt.RequestID)
	return &models.CVPreparedEvent{
		RequestID: evt.RequestID,
		CVID:      id,
		UploadURL: uploadURL.String(),
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

// index runs the full pipeline: download → extract → embed → qdrant → publish cv.indexed.
func (s *service) index(ctx context.Context, evt models.CVIndexRequestedEvent) error {
	log := logrus.WithField("cv_id", evt.CVID)

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, evt.CVID, &cv); err != nil {
		// CV not found is a permanent failure — ACK so it is not retried.
		log.Warnf("cv not found, discarding: %v", err)
		return nil
	}
	switch cv.Status {
	case models.CVStatusIndexed, models.CVStatusIndexing, models.CVStatusNormalizing, models.CVStatusNormalized:
		log.Infof("already %s, skipping", cv.Status)
		return nil
	case models.CVStatusFailed:
		log.Warnf("cv previously failed (%s), discarding", cv.ErrorMsg)
		return nil
	}

	// fail marks the CV as failed and ACKs (permanent failure, no retry).
	fail := func(msg string) error {
		_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
			"status": models.CVStatusFailed, "error": msg, "updated_at": time.Now(),
		})
		log.Errorf("cv failed: %s", msg)
		return nil
	}

	// 1. Verify file exists in MinIO — source of truth for upload completion.
	info, err := infra.GetMinio().StatObject(ctx, cv.MinioBucket, cv.MinioKey, minio.StatObjectOptions{})
	if err != nil {
		// Object not found means file was never uploaded — permanent failure.
		// Any other MinIO error is transient — retry.
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return fail("file not uploaded to storage")
		}
		return fmt.Errorf("stat minio object: %w", err)
	}

	userID, err := s.upsertUser(ctx, evt.Email)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err) // transient — retry
	}

	// Replace any previous CV for this user (MongoDB + MinIO + Qdrant).
	if err := s.replaceExistingCV(ctx, userID, cv.ID, log); err != nil {
		return fmt.Errorf("replace existing cv: %w", err) // transient — retry
	}

	_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
		"user_id": userID, "status": models.CVStatusIndexing, "error": "", "updated_at": time.Now(),
	})

	// 2. Download from MinIO.
	obj, err := infra.GetMinio().GetObject(ctx, cv.MinioBucket, cv.MinioKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("download from minio: %w", err) // transient — retry
	}
	defer obj.Close()

	// 3. Extract text — use size from StatObject.
	text, err := extractText(obj, info.Size)
	if err != nil {
		return fail(fmt.Sprintf("extract text: %v", err)) // bad file — permanent
	}
	if text == "" {
		return fail("document appears to be empty")
	}
	log.Infof("extracted %d chars from %s", len(text), cv.Filename)

	// 4. Embed — transient failure, retry.
	vector, err := s.embeddings.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	log.Infof("embedding generated (%d dims)", len(vector))

	// 5. Upsert into Qdrant — transient failure, retry.
	qdrantID := uuid.New().String()
	payload := map[string]string{"cv_id": cv.ID, "user_id": userID, "filename": cv.Filename}
	if err := s.qdrant.Upsert(ctx, qdrantID, vector, payload); err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}

	// 6. Mark indexed in MongoDB.
	_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
		"status":         models.CVStatusIndexed,
		"qdrant_id":      qdrantID,
		"extracted_text": text,
		"error":          "",
		"updated_at":     time.Now(),
	})

	// 7. Publish cv.indexed and transition to normalizing.
	out := models.CVIndexedEvent{CVID: cv.ID, UserID: userID, QdrantID: qdrantID}
	if err := system.Publish(ctx, constants.SubjectCVIndexed, out); err != nil {
		log.Warnf("publish cv.indexed: %v", err)
	} else {
		log.Infof("published cv.indexed — status → normalizing")
		_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
			"status": models.CVStatusNormalizing, "updated_at": time.Now(),
		})
	}

	return nil
}

// replaceExistingCV deletes all previous CVs for userID from MongoDB, MinIO, and Qdrant,
// excluding the CV that is currently being indexed (currentCVID).
func (s *service) replaceExistingCV(ctx context.Context, userID, currentCVID string, log *logrus.Entry) error {
	var previous []models.PersistedCV
	if err := system.GetStorage().GetManyByField(ctx, constants.MongoCVsCollection, "user_id", []string{userID}, &previous); err != nil {
		return fmt.Errorf("query previous cvs: %w", err)
	}

	for _, old := range previous {
		if old.ID == currentCVID {
			continue
		}

		// Delete from MinIO.
		if old.MinioKey != "" {
			if err := infra.GetMinio().RemoveObject(ctx, old.MinioBucket, old.MinioKey, minio.RemoveObjectOptions{}); err != nil {
				log.Warnf("remove old minio object %s: %v", old.MinioKey, err)
			}
		}

		// Delete from Qdrant.
		if old.QdrantID != "" {
			if err := s.qdrant.Delete(ctx, old.QdrantID); err != nil {
				log.Warnf("delete old qdrant point %s: %v", old.QdrantID, err)
			}
		}

		// Delete from MongoDB.
		if err := system.GetStorage().DeleteByField(ctx, constants.MongoCVsCollection, "id", old.ID); err != nil {
			log.Warnf("delete old cv doc %s: %v", old.ID, err)
		} else {
			log.Infof("replaced previous cv %s for user %s", old.ID, userID)
		}
	}

	return nil
}

func (s *service) upsertUser(ctx context.Context, email string) (string, error) {
	var user models.User
	if err := system.GetStorage().Get(ctx, constants.MongoUsersCollection, bson.M{"email": email}, &user); err == nil {
		return user.ID, nil
	}
	now := time.Now()
	user = models.User{ID: gonanoid.Must(), Email: email, CreatedAt: now, UpdatedAt: now}
	if err := system.GetStorage().SetById(ctx, constants.MongoUsersCollection, user.ID, &user); err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}
