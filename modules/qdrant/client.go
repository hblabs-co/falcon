package qdrant

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
)

// Client wraps the Qdrant HTTP API for a single collection.
type Client struct {
	http       *ownhttp.Client
	collection string
	vectorDim  int
}

// SearchResult is a single match returned by Search.
type SearchResult struct {
	ID      string            `json:"id"`
	Score   float32           `json:"score"`
	Payload map[string]string `json:"payload"`
}

// GetEvent builds a MatchPending event for the given project context.
func (r SearchResult) GetEvent(projectID, platform string) models.MatchPendingEvent {
	return models.MatchPendingEvent{
		CVID:       r.Payload["cv_id"],
		QdrantID:   r.ID,
		UserID:     r.Payload["user_id"],
		ProjectID:  projectID,
		Platform:   platform,
		Similarity: r.Score,
	}
}

// NewFromEnv creates a Client from environment variables.
// QDRANT_URL and QDRANT_COLLECTION are required.
// QDRANT_VECTOR_DIM is optional (required only if EnsureCollection will be called).
func NewFromEnv() (*Client, error) {
	values, err := helpers.ReadEnvs("QDRANT_URL", "QDRANT_COLLECTION")
	if err != nil {
		return nil, err
	}
	url, collection := values[0], values[1]

	var vectorDim int
	if dimStr := helpers.ReadEnvOptional("QDRANT_VECTOR_DIM", ""); dimStr != "" {
		vectorDim, err = strconv.Atoi(dimStr)
		if err != nil {
			return nil, fmt.Errorf("QDRANT_VECTOR_DIM must be an integer: %w", err)
		}
	}

	return &Client{
		http:       ownhttp.New(url, nil),
		collection: collection,
		vectorDim:  vectorDim,
	}, nil
}

// EnsureCollection creates the Qdrant collection if it does not already exist.
// Requires QDRANT_VECTOR_DIM to be set.
func (c *Client) EnsureCollection(ctx context.Context) error {
	if c.vectorDim == 0 {
		return fmt.Errorf("EnsureCollection requires QDRANT_VECTOR_DIM to be set")
	}

	path := "/collections/" + c.collection
	status, err := c.http.Status(ctx, path)
	if err != nil {
		return fmt.Errorf("qdrant check collection: %w", err)
	}
	if status == http.StatusOK {
		return nil
	}

	return c.http.Put(ctx, path, ownhttp.Request{
		Body: map[string]any{
			"vectors": map[string]any{
				"size":     c.vectorDim,
				"distance": "Cosine",
			},
		},
	})
}

// Point is a single (id, vector, payload) tuple for bulk upserts.
type Point struct {
	ID      string            `json:"id"`
	Vector  []float32         `json:"vector"`
	Payload map[string]string `json:"payload"`
}

// Upsert stores or replaces a single point in Qdrant. id must be a UUID.
func (c *Client) Upsert(ctx context.Context, id string, vector []float32, payload map[string]string) error {
	return c.UpsertMany(ctx, []Point{{ID: id, Vector: vector, Payload: payload}})
}

// UpsertMany stores or replaces multiple points in one request. Used by
// the multi-vector indexing path (one point per CV chunk) so a full
// re-indexing of a single CV is one round-trip instead of N.
func (c *Client) UpsertMany(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	path := "/collections/" + c.collection + "/points"
	return c.http.Put(ctx, path, ownhttp.Request{
		Body: map[string]any{"points": points},
	})
}

// Delete removes a single point from Qdrant by its UUID. A no-op if the point does not exist.
func (c *Client) Delete(ctx context.Context, id string) error {
	path := "/collections/" + c.collection + "/points/delete"
	return c.http.Post(ctx, path, ownhttp.Request{
		Body: map[string]any{"points": []string{id}},
	})
}

// DeleteByPayload removes every point whose payload field matches the
// given value. The cleanup path for multi-vector storage: when a user
// re-uploads a CV, we delete all chunks of the previous version with
// one filter call instead of tracking N individual IDs.
func (c *Client) DeleteByPayload(ctx context.Context, field, value string) error {
	path := "/collections/" + c.collection + "/points/delete"
	return c.http.Post(ctx, path, ownhttp.Request{
		Body: map[string]any{
			"filter": map[string]any{
				"must": []map[string]any{
					{"key": field, "match": map[string]any{"value": value}},
				},
			},
		},
	})
}

// Search returns the top matches for vector above scoreThreshold.
func (c *Client) Search(ctx context.Context, vector []float32, limit int, scoreThreshold float32) ([]SearchResult, error) {
	var resp struct {
		Result []SearchResult `json:"result"`
	}

	if err := c.http.Post(ctx, "/collections/"+c.collection+"/points/search", ownhttp.Request{
		Body: map[string]any{
			"vector":          vector,
			"limit":           limit,
			"score_threshold": scoreThreshold,
			"with_payload":    true,
		},
		Result: &resp,
	}); err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}

	return resp.Result, nil
}
