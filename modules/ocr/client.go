package ocr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/ownhttp"
)

// Client calls Mistral's OCR endpoint (or any API-compatible alternative
// configured via OCR_URL). Returns structured text extracted from a
// PDF — handles multi-column layouts and scanned-but-text-layer PDFs
// well enough that a pure-Go library like pdfcpu would otherwise miss.
//
// Payload shape matches Mistral's spec:
//
//	POST /v1/ocr
//	{ "model": "mistral-ocr-latest",
//	  "document": { "type": "document_url", "document_url": "<url>" } }
type Client struct {
	http  *ownhttp.Client
	model string
}

// NewFromEnv creates a Client from OCR_URL, OCR_API_KEY, OCR_MODEL.
// OCR_URL includes the path (e.g. "https://api.mistral.ai/v1/ocr") so
// this client can be pointed at self-hosted equivalents in the future
// without a code change.
func NewFromEnv() (*Client, error) {
	values, err := environment.ReadMany("OCR_URL", "OCR_API_KEY", "OCR_MODEL")
	if err != nil {
		return nil, err
	}
	url, key, model := values[0], values[1], values[2]
	return &Client{
		http:  ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model: model,
	}, nil
}

// ocrResponse mirrors the relevant fields of Mistral's response.
// Extra fields (images, dimensions, etc.) are ignored — we only need
// the per-page markdown text for downstream embedding.
type ocrResponse struct {
	Pages []struct {
		Index    int    `json:"index"`
		Markdown string `json:"markdown"`
	} `json:"pages"`
	Model string `json:"model"`
}

// ExtractFromURL sends `documentURL` to the OCR provider and returns
// the concatenated markdown text across all pages. `documentURL`
// must be reachable by the provider — for PDFs stored in a private
// MinIO bucket, pass a presigned URL generated against the bucket's
// public hostname.
//
// Returns an error for transient failures (timeout, 5xx) so the
// caller can distinguish from "no text extracted" (empty string
// without error — usually means the PDF is image-only and needs a
// different pipeline).
func (c *Client) ExtractFromURL(ctx context.Context, documentURL string, fields ...map[string]any) (string, error) {
	start := time.Now()

	body := map[string]any{
		"model": c.model,
		"document": map[string]any{
			"type":         "document_url",
			"document_url": documentURL,
		},
	}

	var resp ocrResponse
	if err := c.http.Post(ctx, "", ownhttp.Request{
		Body:   body,
		Result: &resp,
	}); err != nil {
		return "", fmt.Errorf("ocr request: %w", err)
	}

	var sb strings.Builder
	for i, p := range resp.Pages {
		if i > 0 {
			// Page-break marker. Keeps the pipeline downstream from
			// silently gluing "experience 2020" + "references" into
			// one paragraph when the chunker splits on blank lines.
			sb.WriteString("\n\n")
		}
		sb.WriteString(p.Markdown)
	}
	text := sb.String()

	logOCR("OCR extract", start, len(text), len(resp.Pages), fields)
	return text, nil
}

func logOCR(msg string, start time.Time, chars, pages int, extra []map[string]any) {
	f := logrus.Fields{
		"took":  time.Since(start).String(),
		"chars": chars,
		"pages": pages,
	}
	for _, m := range extra {
		for k, v := range m {
			f[k] = v
		}
	}
	logrus.WithFields(f).Info(msg)
}
