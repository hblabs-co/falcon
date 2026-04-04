package cv

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/ownhttp"
)

type qdrantClient struct {
	http       *ownhttp.Client
	collection string
	vectorDim  int
}

func newQdrantClient() (*qdrantClient, error) {
	values, err := helpers.ReadEnvs("QDRANT_URL", "QDRANT_COLLECTION", "QDRANT_VECTOR_DIM")
	if err != nil {
		return nil, err
	}

	url, collection, dimStr := values[0], values[1], values[2]
	dim, err := strconv.Atoi(dimStr)
	if err != nil {
		return nil, fmt.Errorf("QDRANT_VECTOR_DIM must be an integer: %w", err)
	}
	return &qdrantClient{
		http:       ownhttp.New(url, nil),
		collection: collection,
		vectorDim:  dim,
	}, nil
}

// EnsureCollection creates the Qdrant collection if it does not already exist.
func (q *qdrantClient) EnsureCollection(ctx context.Context) error {
	path := "/collections/" + q.collection

	status, err := q.http.Status(ctx, path)
	if err != nil {
		return fmt.Errorf("qdrant check collection: %w", err)
	}
	if status == http.StatusOK {
		return nil
	}

	req := ownhttp.Request{
		Body: map[string]any{
			"vectors": map[string]any{
				"size":     q.vectorDim,
				"distance": "Cosine",
			},
		},
	}

	return q.http.Put(ctx, path, req)
}

type qdrantPoint struct {
	ID      string            `json:"id"`
	Vector  []float32         `json:"vector"`
	Payload map[string]string `json:"payload"`
}

// Upsert stores or replaces a point (CV vector) in Qdrant.
func (q *qdrantClient) Upsert(ctx context.Context, id string, vector []float32, payload map[string]string) error {
	path := "/collections/" + q.collection + "/points"
	req := ownhttp.Request{
		Body: map[string]any{"points": []qdrantPoint{{ID: id, Vector: vector, Payload: payload}}},
	}
	return q.http.Put(ctx, path, req)
}
