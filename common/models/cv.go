package models

import "time"

// CVStatus tracks the lifecycle of a CV through the ingest pipeline.
type CVStatus string

const (
	CVStatusPendingUpload CVStatus = "pending_upload"
	CVStatusUploaded      CVStatus = "uploaded"
	CVStatusIndexing      CVStatus = "indexing"
	CVStatusIndexed       CVStatus = "indexed"
	CVStatusFailed        CVStatus = "failed"
)

// PersistedCV is the MongoDB document for a user CV.
type PersistedCV struct {
	ID          string    `json:"id"           bson:"id"`
	UserID      string    `json:"user_id"      bson:"user_id"`
	Filename    string    `json:"filename"     bson:"filename"`
	MinioBucket string    `json:"minio_bucket" bson:"minio_bucket"`
	MinioKey    string    `json:"minio_key"    bson:"minio_key"`
	Status        CVStatus  `json:"status"                  bson:"status"`
	ErrorMsg      string    `json:"error,omitempty"         bson:"error,omitempty"`
	QdrantID      string    `json:"qdrant_id,omitempty"     bson:"qdrant_id,omitempty"`
	ExtractedText string    `json:"extracted_text,omitempty" bson:"extracted_text,omitempty"`
	CreatedAt   time.Time `json:"created_at"   bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"   bson:"updated_at"`
}

// CVIndexedEvent is published to NATS subject "cv.indexed" once a CV has been
// fully processed and its vector stored in Qdrant.
type CVIndexedEvent struct {
	CVID     string `json:"cv_id"`
	UserID   string `json:"user_id"`
	QdrantID string `json:"qdrant_id"`
}
