package embeddings

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/ownhttp"
)

// Client calls an OpenAI-compatible embeddings API.
type Client struct {
	http  *ownhttp.Client
	model string
}

// NewFromEnv creates a Client from environment variables.
// EMBEDDINGS_URL, EMBEDDINGS_API_KEY, and EMBEDDINGS_MODEL are required.
func NewFromEnv() (*Client, error) {
	values, err := helpers.ReadEnvs("EMBEDDINGS_URL", "EMBEDDINGS_API_KEY", "EMBEDDINGS_MODEL")
	if err != nil {
		return nil, err
	}
	url, key, model := values[0], values[1], values[2]
	return &Client{
		http:  ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model: model,
	}, nil
}

// chunkTargetChars is the approximate size each chunk aims for when
// EmbedChunks splits a document. ~500 chars ≈ 125-150 tokens — a
// section or two bullets of a CV, small enough for embeddings to
// capture specific skills / experiences without dilution, big enough
// for the model to see meaningful context. Tunable per-caller if
// needed (the current call site uses this default).
const chunkTargetChars = 500

// maxChunkChars is the hard cap per single API request. Used by the
// legacy whole-document Embed() path and as a fallback when a single
// paragraph exceeds chunkTargetChars. Stays well below the 8192-token
// Mistral context window.
const maxChunkChars = 20000

// paragraphSplit is a blank-line delimiter — the most reliable
// structural cue in extracted .docx text. Single newlines often sit
// mid-sentence (bullets, wrapped lines) so we only split on DOUBLE
// newlines or more.
var paragraphSplit = regexp.MustCompile(`\n\s*\n+`)

// Embed returns a single embedding for the given text. For inputs that
// exceed the provider's context window we chunk on rune boundaries and
// return the element-wise mean of the chunks' vectors. Not ideal for
// retrieval-per-section use cases (switch to multi-vector storage for
// that), but good enough for whole-document similarity.
//
// Optional fields are merged into the structured log line so callers
// fired in a loop (e.g. dispatch's backfill) can attach project_id /
// cv_id for traceability.
func (c *Client) Embed(ctx context.Context, text string, fields ...map[string]any) ([]float32, error) {
	start := time.Now()

	chunks := splitRunes(text, maxChunkChars)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("embed: empty input")
	}

	// Fast path: single chunk avoids the allocation + loop.
	if len(chunks) == 1 {
		v, err := c.embedOne(ctx, chunks[0])
		if err != nil {
			return nil, err
		}
		logEmbed("Embed", start, len(text), 1, fields)
		return v, nil
	}

	// Chunked path: mean-pool each chunk's vector. Dimension is taken
	// from the first response — all chunks come from the same model so
	// dims match by construction.
	var sum []float32
	for i, chunk := range chunks {
		v, err := c.embedOne(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", i+1, len(chunks), err)
		}
		if sum == nil {
			sum = make([]float32, len(v))
		}
		for j, x := range v {
			sum[j] += x
		}
	}
	inv := 1.0 / float32(len(chunks))
	for i := range sum {
		sum[i] *= inv
	}

	logEmbed("Embed (chunked mean-pool)", start, len(text), len(chunks), fields)
	return sum, nil
}

// logEmbed emits the standard Embed log line with caller-supplied context.
func logEmbed(msg string, start time.Time, chars, chunks int, extra []map[string]any) {
	f := logrus.Fields{
		"took":   time.Since(start).String(),
		"chars":  chars,
		"chunks": chunks,
	}
	for _, m := range extra {
		for k, v := range m {
			f[k] = v
		}
	}
	logrus.WithFields(f).Info(msg)
}

// Chunk is a piece of a longer document with its own embedding. Index
// is the chunk's position in the original text (0-based) — useful for
// debugging and for reconstructing order in UIs.
type Chunk struct {
	Index     int
	Text      string
	Embedding []float32
}

// EmbedChunks splits the text into paragraph-aligned chunks of roughly
// chunkTargetChars, embeds each individually, and returns them with
// their vectors. Used by the multi-vector storage path: callers upsert
// each chunk as a separate point in Qdrant with payload linking back
// to the parent document, so search can score each section on its own
// instead of on a diluted mean.
func (c *Client) EmbedChunks(ctx context.Context, text string) ([]Chunk, error) {
	start := time.Now()

	pieces := splitIntoChunks(text, chunkTargetChars)
	if len(pieces) == 0 {
		return nil, fmt.Errorf("embed chunks: empty input")
	}

	out := make([]Chunk, len(pieces))
	for i, p := range pieces {
		v, err := c.embedOne(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", i+1, len(pieces), err)
		}
		out[i] = Chunk{Index: i, Text: p, Embedding: v}
	}

	logrus.WithFields(logrus.Fields{
		"took":   time.Since(start).String(),
		"chunks": len(out),
		"chars":  len(text),
	}).Info("EmbedChunks")
	return out, nil
}

// splitIntoChunks breaks text into paragraph-aligned pieces of roughly
// targetChars each. Paragraphs (blank-line separated) stay together
// when they fit; consecutive small paragraphs merge until the target
// size is reached. Paragraphs larger than maxChunkChars get split on
// rune boundaries as a last resort (rare in CVs).
func splitIntoChunks(text string, targetChars int) []string {
	paragraphs := paragraphSplit.Split(strings.TrimSpace(text), -1)
	var chunks []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// A paragraph so large it alone blows past maxChunkChars: emit
		// what we have and split the monster on runes. Keeps us within
		// the API limit even when CVs have one giant blob.
		if len([]rune(p)) > maxChunkChars {
			flush()
			for _, piece := range splitRunes(p, maxChunkChars) {
				chunks = append(chunks, piece)
			}
			continue
		}
		// Current chunk would overflow → flush and start fresh with p.
		if current.Len() > 0 && current.Len()+2+len(p) > targetChars {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}
	flush()
	return chunks
}

// embedOne sends a single chunk and returns its embedding.
func (c *Client) embedOne(ctx context.Context, text string) ([]float32, error) {
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := c.http.Post(ctx, "", ownhttp.Request{
		Body:   map[string]string{"input": text, "model": c.model},
		Result: &result,
	}); err != nil {
		return nil, fmt.Errorf("embeddings request: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embeddings API returned empty data")
	}
	return result.Data[0].Embedding, nil
}

// splitRunes chunks s on rune boundaries so each chunk has at most
// maxChars runes. Avoids cutting mid-UTF-8, which would produce
// replacement chars in the provider's tokenizer and skew embeddings.
func splitRunes(s string, maxChars int) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return []string{s}
	}
	chunks := make([]string, 0, (len(runes)+maxChars-1)/maxChars)
	for i := 0; i < len(runes); i += maxChars {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
