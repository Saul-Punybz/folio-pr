// Package storage provides S3-compatible object storage for evidence preservation.
package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/config"
)

// Client wraps an S3-compatible object storage client.
type Client struct {
	s3     *s3.Client
	bucket string
}

// Evidence holds the retrieved evidence artifacts for an article.
type Evidence struct {
	RawHTML   []byte         `json:"raw_html,omitempty"`
	Extracted []byte         `json:"extracted,omitempty"`
	Meta      *CaptureMeta   `json:"meta,omitempty"`
}

// CaptureMeta records metadata about the evidence capture.
type CaptureMeta struct {
	ArticleID   uuid.UUID `json:"article_id"`
	CapturedAt  time.Time `json:"captured_at"`
	RawHash     string    `json:"raw_hash_sha256"`
	ExtractHash string    `json:"extract_hash_sha256"`
	Policy      string    `json:"evidence_policy"`
}

// NewClient creates a new S3-compatible storage client configured for
// Oracle Object Storage (or any S3-compatible endpoint).
func NewClient(ctx context.Context, cfg config.S3Config) (*Client, error) {
	if cfg.Endpoint == "" {
		slog.Warn("S3 endpoint not configured, evidence storage disabled")
		return &Client{bucket: cfg.Bucket}, nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = &cfg.Endpoint
		o.UsePathStyle = true
	})

	return &Client{
		s3:     client,
		bucket: cfg.Bucket,
	}, nil
}

// Configured returns true if the S3 client has a valid connection configured.
func (c *Client) Configured() bool {
	return c.s3 != nil
}

// StoreEvidence compresses and uploads the raw HTML, extracted text, and
// capture metadata for an article to S3-compatible object storage.
func (c *Client) StoreEvidence(ctx context.Context, articleID uuid.UUID, policy string, rawHTML []byte, extracted []byte, meta []byte) error {
	if c.s3 == nil {
		slog.Warn("evidence storage not configured, skipping upload", "article_id", articleID)
		return nil
	}

	prefix := fmt.Sprintf("evidence/%s/%s", policy, articleID)

	// Compute content hashes.
	rawHash := sha256sum(rawHTML)
	extractHash := sha256sum(extracted)

	// Build capture_meta.json.
	captureMeta := CaptureMeta{
		ArticleID:   articleID,
		CapturedAt:  time.Now().UTC(),
		RawHash:     rawHash,
		ExtractHash: extractHash,
		Policy:      policy,
	}
	metaJSON, err := json.MarshalIndent(captureMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: marshal meta: %w", err)
	}

	// Upload each artifact.
	uploads := map[string][]byte{
		prefix + "/raw.html.gz":      rawHTML,
		prefix + "/extracted.txt.gz": extracted,
		prefix + "/capture_meta.json": metaJSON,
	}

	for key, data := range uploads {
		var body []byte
		if key == prefix+"/capture_meta.json" {
			// Meta is not compressed.
			body = data
		} else {
			compressed, err := gzipCompress(data)
			if err != nil {
				return fmt.Errorf("storage: compress %s: %w", key, err)
			}
			body = compressed
		}

		_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &c.bucket,
			Key:    &key,
			Body:   bytes.NewReader(body),
		})
		if err != nil {
			return fmt.Errorf("storage: upload %s: %w", key, err)
		}

		slog.Debug("evidence uploaded", "key", key, "size", len(body))
	}

	return nil
}

// DeleteEvidence removes all evidence artifacts for an article across all
// retention policy prefixes.
func (c *Client) DeleteEvidence(ctx context.Context, articleID uuid.UUID) error {
	if c.s3 == nil {
		slog.Warn("evidence storage not configured, skipping delete", "article_id", articleID)
		return nil
	}

	policies := []string{"ret_3m", "ret_6m", "ret_12m", "keep"}
	suffixes := []string{"/raw.html.gz", "/extracted.txt.gz", "/capture_meta.json"}

	for _, policy := range policies {
		prefix := fmt.Sprintf("evidence/%s/%s", policy, articleID)
		for _, suffix := range suffixes {
			key := prefix + suffix
			_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &c.bucket,
				Key:    &key,
			})
			if err != nil {
				// Log but don't fail on individual object deletions — the
				// object may not exist under this policy prefix.
				slog.Debug("evidence delete (may not exist)", "key", key, "err", err)
			}
		}
	}

	slog.Debug("evidence deleted", "article_id", articleID)
	return nil
}

// GetEvidence retrieves all evidence artifacts for an article.
// It tries all retention policy prefixes and returns the first match.
func (c *Client) GetEvidence(ctx context.Context, articleID uuid.UUID) (*Evidence, error) {
	if c.s3 == nil {
		return nil, fmt.Errorf("storage: not configured")
	}

	policies := []string{"ret_3m", "ret_6m", "ret_12m", "keep"}

	for _, policy := range policies {
		prefix := fmt.Sprintf("evidence/%s/%s", policy, articleID)
		ev, err := c.fetchEvidence(ctx, prefix)
		if err == nil {
			return ev, nil
		}
	}

	return nil, fmt.Errorf("storage: no evidence found for article %s", articleID)
}

func (c *Client) fetchEvidence(ctx context.Context, prefix string) (*Evidence, error) {
	ev := &Evidence{}

	// Raw HTML.
	rawData, err := c.getObject(ctx, prefix+"/raw.html.gz")
	if err != nil {
		return nil, err
	}
	ev.RawHTML, err = gzipDecompress(rawData)
	if err != nil {
		return nil, fmt.Errorf("storage: decompress raw: %w", err)
	}

	// Extracted text.
	extData, err := c.getObject(ctx, prefix+"/extracted.txt.gz")
	if err != nil {
		return nil, err
	}
	ev.Extracted, err = gzipDecompress(extData)
	if err != nil {
		return nil, fmt.Errorf("storage: decompress extracted: %w", err)
	}

	// Meta.
	metaData, err := c.getObject(ctx, prefix+"/capture_meta.json")
	if err != nil {
		return nil, err
	}
	var meta CaptureMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("storage: unmarshal meta: %w", err)
	}
	ev.Meta = &meta

	return ev, nil
}

func (c *Client) getObject(ctx context.Context, key string) ([]byte, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("storage: read %s: %w", key, err)
	}
	return data, nil
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
