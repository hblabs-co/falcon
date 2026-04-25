package models

import "time"

// CVStatus tracks the lifecycle of a CV through the ingest pipeline.
type CVStatus string

const (
	CVStatusPendingUpload CVStatus = "pending_upload"
	CVStatusUploaded      CVStatus = "uploaded"
	CVStatusIndexing      CVStatus = "indexing"
	CVStatusIndexed       CVStatus = "indexed"
	CVStatusNormalizing   CVStatus = "normalizing"
	CVStatusNormalized    CVStatus = "normalized"
	CVStatusFailed        CVStatus = "failed"
)

// PersistedCV is the MongoDB document for a user CV.
type PersistedCV struct {
	ID            string        `json:"id"           bson:"id"`
	UserID        string        `json:"user_id"      bson:"user_id"`
	Filename      string        `json:"filename"     bson:"filename"`
	MinioBucket   string        `json:"minio_bucket" bson:"minio_bucket"`
	MinioKey      string        `json:"minio_key"    bson:"minio_key"`
	Status        CVStatus      `json:"status"                   bson:"status"`
	ErrorMsg      string        `json:"error,omitempty"          bson:"error,omitempty"`
	QdrantID      string        `json:"qdrant_id,omitempty"      bson:"qdrant_id,omitempty"`
	ExtractedText string        `json:"extracted_text,omitempty" bson:"extracted_text,omitempty"`
	Normalized    *NormalizedCV `json:"normalized,omitempty"    bson:"normalized,omitempty"`
	CreatedAt     time.Time     `json:"created_at"   bson:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"   bson:"updated_at"`
}

// NormalizedCV holds the trilingual normalized representation of a CV.
type NormalizedCV struct {
	De NormalizedCVLang `json:"de" bson:"de"`
	En NormalizedCVLang `json:"en" bson:"en"`
	Es NormalizedCVLang `json:"es" bson:"es"`
}

// NormalizedCVLang is the language-specific normalized CV content.
type NormalizedCVLang struct {
	FirstName    string              `json:"first_name"   bson:"first_name"`
	LastName     string              `json:"last_name"    bson:"last_name"`
	Summary      string              `json:"summary"      bson:"summary"`
	Experience   []CVExperienceEntry `json:"experience"   bson:"experience"`
	Technologies CVTechnologies      `json:"technologies" bson:"technologies"`
}

// CVExperienceEntry represents one position in a candidate's work history.
type CVExperienceEntry struct {
	Company          string   `json:"company"           bson:"company"`
	Role             string   `json:"role"              bson:"role"`
	Start            string   `json:"start"             bson:"start"`
	End              string   `json:"end"               bson:"end"`
	Duration         string   `json:"duration"          bson:"duration"`
	ShortDescription string   `json:"short_description" bson:"short_description"`
	LongDescription  string   `json:"long_description"  bson:"long_description"`
	Highlights       []string `json:"highlights"        bson:"highlights"`
	Tasks            []string `json:"tasks"             bson:"tasks"`
	Technologies     []string `json:"technologies"      bson:"technologies"`
}

// CVTechnologies categorises all technologies found across the entire CV.
type CVTechnologies struct {
	Frontend  []string `json:"frontend"   bson:"frontend"`
	Backend   []string `json:"backend"    bson:"backend"`
	Databases []string `json:"databases"  bson:"databases"`
	DevOps    []string `json:"devops"     bson:"devops"`
	Tools     []string `json:"tools"      bson:"tools"`
	Others    []string `json:"others"     bson:"others"`
}

// CVIndexedEvent is published to NATS subject "cv.indexed" once a CV has been
// fully processed and its vector stored in Qdrant.
type CVIndexedEvent struct {
	CVID     string `json:"cv_id"`
	UserID   string `json:"user_id"`
	QdrantID string `json:"qdrant_id"`
}
