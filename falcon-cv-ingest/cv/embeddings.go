package cv

import (
	"context"
	"fmt"

	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/ownhttp"
)

type embeddingsClient struct {
	http  *ownhttp.Client
	model string
}

func newEmbeddingsClient() (*embeddingsClient, error) {
	values, err := helpers.ReadEnvs("EMBEDDINGS_URL", "EMBEDDINGS_API_KEY", "EMBEDDINGS_MODEL")
	if err != nil {
		return nil, err
	}

	url, key, model := values[0], values[1], values[2]
	c := &embeddingsClient{
		http:  ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model: model,
	}

	return c, nil
}

// Embed returns the embedding vector for the given text.
func (c *embeddingsClient) Embed(ctx context.Context, text string) ([]float32, error) {
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	req := ownhttp.Request{
		Body:   map[string]string{"input": text, "model": c.model},
		Result: &result,
	}

	err := c.http.Post(ctx, "", req)
	if err != nil {
		return nil, fmt.Errorf("embeddings request: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embeddings API returned empty data")
	}

	return result.Data[0].Embedding, nil
}
