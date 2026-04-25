package cv

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/embeddings"
	"hblabs.co/falcon/storage/infra"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/ocr"
	"hblabs.co/falcon/packages/qdrant"
	"hblabs.co/falcon/packages/system"
)

const (
	cvsBucket          = "cvs"
	presignedURLExpiry = 15 * time.Minute
	// presignedOCRExpiry is how long the GET URL handed to Mistral
	// OCR stays valid. Kept short — OCR requests that take longer
	// than 10 min are usually stuck and should fail clean instead
	// of succeeding against a still-open URL.
	presignedOCRExpiry = 10 * time.Minute
	// presignedDownloadExpiry — admin "download CV" link. Long
	// enough to click "Save as", short enough that it can't be
	// casually shared.
	presignedDownloadExpiry = 5 * time.Minute
)

type service struct {
	embeddings *embeddings.Client
	qdrant     *qdrant.Client
	ocr        *ocr.Client
}

func newService(ctx context.Context) (*service, error) {
	if err := infra.EnsureBucket(ctx, cvsBucket, false); err != nil {
		return nil, err
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

	ocrClient, err := ocr.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("ocr client: %w", err)
	}

	return &service{embeddings: emb, qdrant: qdr, ocr: ocrClient}, nil
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

// prepareDownload looks up the user's CV in Mongo and returns a
// presigned MinIO GET URL for the file. Used by falcon-admin
// for the admin "download CV" button — keeps MinIO access on this
// service instead of leaking the client to every consumer.
//
// Returns NotFound=true (with empty URL) when the user has no CV
// or the CV record lacks a minio key, so callers don't have to
// distinguish "no record" from "presign error" themselves.
func (s *service) prepareDownload(ctx context.Context, evt models.CVDownloadRequestedEvent) (*models.CVDownloadPreparedEvent, error) {
	if evt.UserID == "" {
		return &models.CVDownloadPreparedEvent{RequestID: evt.RequestID, NotFound: true}, nil
	}

	var cv models.PersistedCV
	if err := system.GetStorage().GetByField(ctx, constants.MongoCVsCollection,
		"user_id", evt.UserID, &cv); err != nil {
		// Treat any read failure (typically: doc not found) as
		// "no CV". The mongo driver returns the same error shape
		// either way; the caller only needs the boolean.
		return &models.CVDownloadPreparedEvent{RequestID: evt.RequestID, NotFound: true}, nil
	}
	if cv.MinioBucket == "" || cv.MinioKey == "" {
		return &models.CVDownloadPreparedEvent{RequestID: evt.RequestID, NotFound: true}, nil
	}

	reqParams := url.Values{}
	if cv.Filename != "" {
		reqParams.Set("response-content-disposition",
			`attachment; filename="`+sanitizeFilename(cv.Filename)+`"`)
	}
	signed, err := infra.GetMinioPublic().PresignedGetObject(ctx,
		cv.MinioBucket, cv.MinioKey, presignedDownloadExpiry, reqParams)
	if err != nil {
		return nil, fmt.Errorf("presign cv download: %w", err)
	}

	expiresAt := time.Now().Add(presignedDownloadExpiry)
	logrus.Infof("[cv] presigned download user_id=%s key=%s request_id=%s",
		evt.UserID, cv.MinioKey, evt.RequestID)
	return &models.CVDownloadPreparedEvent{
		RequestID: evt.RequestID,
		URL:       signed.String(),
		Filename:  cv.Filename,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

// sanitizeFilename strips characters that would break the
// Content-Disposition header. MinIO/S3 quotes the value, but a
// stray quote in the original filename would still escape.
func sanitizeFilename(s string) string {
	r := strings.NewReplacer(`"`, "", `\`, "", "\n", "", "\r", "")
	return r.Replace(s)
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

	// 2+3. Extract text — dispatch by file type.
	//   .pdf  → presign a GET URL against the public MinIO endpoint
	//           and hand it to Mistral OCR. The bucket stays private;
	//           the URL is short-lived (10 min) and signed. Mistral
	//           downloads from outside the cluster via the public
	//           ingress, same way users' browsers would.
	//   .docx → stream the object locally and parse the Word XML.
	//           Faster + cheaper (no external API round-trip) and
	//           works offline.
	var text string
	if isPDF(cv.Filename) {
		presigned, err := infra.GetMinioPublic().PresignedGetObject(
			ctx, cv.MinioBucket, cv.MinioKey, presignedOCRExpiry, url.Values{},
		)
		if err != nil {
			return fmt.Errorf("presign GET: %w", err) // transient — retry
		}
		text, err = s.ocr.ExtractFromURL(ctx, presigned.String(), map[string]any{
			"cv_id":    cv.ID,
			"filename": cv.Filename,
		})
		if err != nil {
			return fmt.Errorf("ocr extract: %w", err) // transient — retry (Mistral may be 5xx)
		}
	} else {
		obj, err := infra.GetMinio().GetObject(ctx, cv.MinioBucket, cv.MinioKey, minio.GetObjectOptions{})
		if err != nil {
			return fmt.Errorf("download from minio: %w", err) // transient — retry
		}
		defer obj.Close()
		text, err = extractDOCXText(obj, info.Size)
		if err != nil {
			return fail(fmt.Sprintf("extract text: %v", err)) // bad file — permanent
		}
	}
	if text == "" {
		if isPDF(cv.Filename) {
			return fail("PDF has no selectable text — export from Word or use a text-layer PDF")
		}
		return fail("document appears to be empty")
	}
	log.Infof("extracted %d chars from %s", len(text), cv.Filename)

	// 4. Embed — split into paragraph-aligned chunks and embed each.
	//    Multi-vector layout in Qdrant: one point per chunk, all sharing
	//    payload.cv_id so downstream search can group chunks by CV. Clean
	//    up any prior run first so we don't leave stale points when a
	//    retry happens after a partial failure.
	if err := s.qdrant.DeleteByPayload(ctx, "cv_id", cv.ID); err != nil {
		log.Warnf("pre-clean qdrant chunks for cv=%s: %v", cv.ID, err)
	}
	chunks, err := s.embeddings.EmbedChunks(ctx, text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	log.Infof("embedded %d chunk(s)", len(chunks))

	// 5. Bulk upsert all chunks in one request.
	points := make([]qdrant.Point, len(chunks))
	for i, ch := range chunks {
		points[i] = qdrant.Point{
			ID:     uuid.New().String(),
			Vector: ch.Embedding,
			Payload: map[string]string{
				"cv_id":     cv.ID,
				"user_id":   userID,
				"filename":  cv.Filename,
				"chunk_idx": strconv.Itoa(ch.Index),
			},
		}
	}
	if err := s.qdrant.UpsertMany(ctx, points); err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}

	// 6. Mark indexed in MongoDB. qdrant_id is no longer a single UUID;
	//    we track just the count for debugging — actual cleanup on next
	//    re-index uses DeleteByPayload(cv_id) instead.
	_ = system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
		"status":         models.CVStatusIndexed,
		"qdrant_chunks":  len(chunks),
		"extracted_text": text,
		"error":          "",
		"updated_at":     time.Now(),
	})

	// 7. Publish cv.indexed and transition to normalizing.
	out := models.CVIndexedEvent{CVID: cv.ID, UserID: userID}
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

		// Delete from Qdrant. With multi-vector storage we don't track
		// individual point IDs anymore; every chunk carries payload.cv_id
		// so one filter call removes them all.
		if err := s.qdrant.DeleteByPayload(ctx, "cv_id", old.ID); err != nil {
			log.Warnf("delete old qdrant chunks for cv %s: %v", old.ID, err)
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
