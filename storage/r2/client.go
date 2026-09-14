// Package r2 is a minimal Cloudflare R2 (S3-compatible) object-storage client
// for services that own a SINGLE bucket. It intentionally exposes only the
// object-level operations a media pipeline needs — put, multipart put, delete —
// and deliberately omits bucket/admin management (create/list/delete bucket).
// Presigned uploads ARE supported (PresignPut): a browser uploading straight to
// the bucket is the point of a lite deployment. Presigned downloads remain out
// of scope — a public bucket plus PublicURL already covers reads.
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsHttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyMiddleware "github.com/aws/smithy-go/middleware"
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

// defaultPresignExpiry bounds how long a minted upload URL stays usable. Short
// on purpose: the URL is a bearer credential for one key, and the client is
// expected to upload immediately after asking for it.
const defaultPresignExpiry = 15 * time.Minute

// ErrNotFound is what Stat returns for a key that does not exist, so callers
// can branch on absence without importing the S3 error types.
var ErrNotFound = errors.New("r2: object not found")

// ObjectInfo is the subset of object metadata a media pipeline verifies after
// a direct browser upload.
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// PresignPut returns a URL that uploads one object under key directly, without
// the caller's credentials ever reaching the browser.
//
// contentType is part of the signature, so an upload whose Content-Type header
// disagrees with it is rejected by R2 — the caller decides what kind of object
// may land at that key, not whoever holds the URL. Size is NOT signed: a plain
// presigned PUT carries no length policy, so a caller that needs a byte cap
// must verify it after the fact with Stat.
func (c *Client) PresignPut(ctx context.Context, key, contentType string, expires time.Duration) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("r2: key is required")
	}
	if strings.TrimSpace(contentType) == "" {
		return "", errors.New("r2: contentType is required")
	}
	if expires <= 0 {
		expires = defaultPresignExpiry
	}

	presignClient := s3.NewPresignClient(c.s3, func(o *s3.PresignOptions) {
		o.Expires = expires
		// The SDK's own PutObject presign path drops the Content-Type header
		// whenever Content-Length is zero — true here by construction, since a
		// presigned PUT is minted before any body exists — via a built-in
		// "RemoveContentTypeHeader" build middleware. Left alone that would
		// silently unsign contentType, contradicting the doc comment above, so
		// it is removed again (best-effort: an absent middleware is not an
		// error) once the SDK has added it.
		o.ClientOptions = append(o.ClientOptions, func(so *s3.Options) {
			so.APIOptions = append(so.APIOptions, func(stack *smithyMiddleware.Stack) error {
				_, _ = stack.Build.Remove("RemoveContentTypeHeader")
				return nil
			})
		})
	})
	request, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("r2: presign put %q: %w", key, err)
	}
	return request.URL, nil
}

// Stat reads one object's metadata. A missing key is ErrNotFound, not a
// generic error, because "the browser never finished the upload" is a normal
// outcome a caller handles rather than an exceptional one.
func (c *Client) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	if strings.TrimSpace(key) == "" {
		return ObjectInfo{}, errors.New("r2: key is required")
	}

	output, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return ObjectInfo{}, ErrNotFound
		}
		// R2 answers a HEAD on a missing key with a bare 404 that does not
		// always unmarshal into types.NotFound; treat the status as decisive.
		var responseError *awsHttp.ResponseError
		if errors.As(err, &responseError) && responseError.HTTPStatusCode() == http.StatusNotFound {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, fmt.Errorf("r2: head %q: %w", key, err)
	}

	info := ObjectInfo{}
	if output.ContentLength != nil {
		info.Size = *output.ContentLength
	}
	if output.ContentType != nil {
		info.ContentType = *output.ContentType
	}
	return info, nil
}
