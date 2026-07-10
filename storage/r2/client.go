// Package r2 is a minimal Cloudflare R2 (S3-compatible) object-storage client
// for services that own a SINGLE bucket. It intentionally exposes only the
// object-level operations a media pipeline needs — put, multipart put, delete —
// and deliberately omits bucket/admin management (create/list/delete bucket,
// presigned downloads), which belong to a full storage service, not a
// single-bucket lite deployment.
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// defaultChunkSize is used by PutMultipart when chunkSize <= 0 (8 MiB). It sits
// above S3's 5 MiB minimum non-final part size, so every part except the last
// satisfies the multipart constraint.
const defaultChunkSize = 8 << 20

// Config configures a Client. All fields are required — the target is one R2
// bucket reached with static credentials against a custom endpoint.
type Config struct {
	// Endpoint is the R2 S3 API endpoint, e.g.
	// "https://<accountid>.r2.cloudflarestorage.com".
	Endpoint string
	// AccessKeyID / SecretAccessKey are the R2 API-token credentials.
	AccessKeyID     string
	SecretAccessKey string
	// Bucket is the single bucket every operation targets.
	Bucket string
}

// Client is a single-bucket R2 client. It is safe for concurrent use.
type Client struct {
	s3     *s3.Client
	bucket string
}

// New validates the config and returns a ready Client. It mirrors the R2 setup
// used elsewhere on the platform: static credentials, region "auto", custom
// base endpoint, path-style addressing.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("r2: Endpoint is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, errors.New("r2: credentials are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("r2: Bucket is required")
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
		awsConfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &Client{
		s3:     client,
		bucket: cfg.Bucket,
	}, nil
}

// Put uploads body under key in a single request. The body is buffered into
// memory first so the request carries a known Content-Length (the S3 signer
// needs a seekable payload). For large files prefer PutMultipart.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("r2: read body: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := c.s3.PutObject(ctx, input); err != nil {
		return fmt.Errorf("r2: put object %q: %w", key, err)
	}
	return nil
}

// PutMultipart streams body under key in parts of chunkSize bytes via an S3
// multipart upload. On any failure it best-effort aborts the multipart upload
// so staged parts are released. chunkSize <= 0 selects defaultChunkSize (8 MiB).
func (c *Client) PutMultipart(ctx context.Context, key string, body io.Reader, contentType string, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}

	createInput := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		createInput.ContentType = aws.String(contentType)
	}
	create, err := c.s3.CreateMultipartUpload(ctx, createInput)
	if err != nil {
		return fmt.Errorf("r2: create multipart upload %q: %w", key, err)
	}
	uploadID := create.UploadId

	// abort releases the staged parts on any error path; its own failure is
	// swallowed since the original error is what the caller must see.
	abort := func() {
		_, _ = c.s3.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(c.bucket),
			Key:      aws.String(key),
			UploadId: uploadID,
		})
	}

	var completed []types.CompletedPart
	buf := make([]byte, chunkSize)
	var partNumber int32 = 1

	for {
		n, readErr := io.ReadFull(body, buf)
		if n > 0 {
			part, err := c.s3.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:     aws.String(c.bucket),
				Key:        aws.String(key),
				UploadId:   uploadID,
				PartNumber: aws.Int32(partNumber),
				Body:       bytes.NewReader(buf[:n]),
			})
			if err != nil {
				abort()
				return fmt.Errorf("r2: upload part %d of %q: %w", partNumber, key, err)
			}
			completed = append(completed, types.CompletedPart{
				ETag:       part.ETag,
				PartNumber: aws.Int32(partNumber),
			})
			partNumber++
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			// io.ErrUnexpectedEOF: final short part read above; EOF: stream ended
			// on a boundary. Either way, done reading.
			break
		}
		if readErr != nil {
			abort()
			return fmt.Errorf("r2: read body for %q: %w", key, readErr)
		}
	}

	if len(completed) == 0 {
		abort()
		return errors.New("r2: no data to upload")
	}

	if _, err := c.s3.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	}); err != nil {
		abort()
		return fmt.Errorf("r2: complete multipart upload %q: %w", key, err)
	}
	return nil
}

// Delete removes the object at key. Deleting a missing key is not an error on
// S3/R2.
func (c *Client) Delete(ctx context.Context, key string) error {
	if _, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("r2: delete object %q: %w", key, err)
	}
	return nil
}

// PublicURL joins a public base origin and an object key into a tokenless,
// directly-servable URL (base with any trailing slash trimmed, then "/", then
// key). It is a pure builder — no network call — so it works without a Client.
func PublicURL(base, key string) string {
	return strings.TrimRight(base, "/") + "/" + key
}
