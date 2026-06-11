package medioa

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
)

// defaultChunkSize is used by UploadChunked when chunkSize <= 0 (8 MiB).
const defaultChunkSize = 8 << 20

// UploadInput describes a file to upload. File is the content source; the
// remaining fields are optional hints sent as multipart form fields.
type UploadInput struct {
	// File is the content to upload. Required.
	File io.Reader
	// FileName is the multipart part filename. Required for streaming readers
	// that have no inherent name; also sent as the file_name override field.
	FileName string
	// ContentType, when set, is used as the file part's Content-Type header.
	ContentType string
	// Ext is the stored extension hint (sent as the ext field).
	Ext string
	// Path is the virtual destination folder (≤3 levels; sent as the path
	// field on single-shot upload and on chunked commit).
	Path string
}

// UploadResult is the decoded data of a successful single-shot upload or
// chunked commit.
type UploadResult struct {
	FileID   string `json:"file_id"`
	Token    string `json:"token"`
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Ext      string `json:"ext"`
}

// uploadChunkResult is the decoded data of a stage call.
type uploadChunkResult struct {
	ChunkID    string `json:"chunk_id"`
	FileID     string `json:"file_id"`
	PartNumber int32  `json:"part_number"`
}

// Upload performs a single-shot multipart upload to
// /api/v1/public/storage/upload and returns the result.
//
// The whole file part is buffered in memory before sending so the request
// carries a known Content-Length. For very large files prefer UploadChunked,
// which streams a bounded buffer per chunk.
func (c *Client) Upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	if in.File == nil {
		return nil, errors.New("medioa: UploadInput.File is required")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := createFilePart(writer, "file", in.FileName, in.ContentType)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, in.File); err != nil {
		return nil, err
	}
	if err := writeOptionalFields(writer, map[string]string{
		"file_name": in.FileName,
		"ext":       in.Ext,
		"path":      in.Path,
	}); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	var result UploadResult
	if err := c.doMultipart(ctx, pathUpload, writer.FormDataContentType(), &buf, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadChunked streams in.File in chunks of chunkSize bytes: it stages the
// first chunk with an empty file_id, captures the server-assigned file_id, then
// stages every remaining chunk under that id and finally commits (file_id,
// path). The commit result is returned.
//
// file_id threading is the crux of the flow: only the FIRST stage call sends an
// empty file_id (which makes the server start a new object and mint the id);
// every subsequent stage call and the commit call reuse that captured id.
//
// chunkSize <= 0 selects defaultChunkSize (8 MiB).
func (c *Client) UploadChunked(ctx context.Context, in UploadInput, chunkSize int) (*UploadResult, error) {
	if in.File == nil {
		return nil, errors.New("medioa: UploadInput.File is required")
	}
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}

	buf := make([]byte, chunkSize)
	fileID := ""
	staged := false

	for {
		n, readErr := io.ReadFull(in.File, buf)
		if n > 0 {
			stage, err := c.stageChunk(ctx, in, fileID, buf[:n])
			if err != nil {
				return nil, err
			}
			// The first stage mints the file_id; thread it through the rest.
			fileID = stage.FileID
			staged = true
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			// io.ErrUnexpectedEOF: the final short chunk was read above; EOF:
			// the stream ended exactly on a boundary. Either way, done reading.
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	if !staged || fileID == "" {
		return nil, errors.New("medioa: no data staged for chunked upload")
	}

	return c.commitChunk(ctx, fileID, in.Path)
}

// stageChunk POSTs one chunk to /upload/stage. fileID is empty on the first
// chunk and the captured id thereafter.
func (c *Client) stageChunk(ctx context.Context, in UploadInput, fileID string, chunk []byte) (*uploadChunkResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := createFilePart(writer, "chunk", in.FileName, in.ContentType)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(chunk); err != nil {
		return nil, err
	}
	if err := writeOptionalFields(writer, map[string]string{
		"file_id":   fileID,
		"file_name": in.FileName,
		"ext":       in.Ext,
	}); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	var result uploadChunkResult
	if err := c.doMultipart(ctx, pathUploadStage, writer.FormDataContentType(), &buf, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// commitChunk POSTs to /upload/commit to finalize a chunked upload.
func (c *Client) commitChunk(ctx context.Context, fileID, path string) (*UploadResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("file_id", fileID); err != nil {
		return nil, err
	}
	if err := writeOptionalFields(writer, map[string]string{
		"path": path,
	}); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	var result UploadResult
	if err := c.doMultipart(ctx, pathUploadCommit, writer.FormDataContentType(), &buf, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// createFilePart creates a form-file part, setting an explicit Content-Type
// when contentType is non-empty (multipart's CreateFormFile hardcodes
// application/octet-stream otherwise).
func createFilePart(writer *multipart.Writer, field, fileName, contentType string) (io.Writer, error) {
	if contentType == "" {
		return writer.CreateFormFile(field, fileName)
	}
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		`form-data; name="` + escapeQuotes(field) + `"; filename="` + escapeQuotes(fileName) + `"`,
	}
	header["Content-Type"] = []string{contentType}
	return writer.CreatePart(header)
}

// writeOptionalFields writes each non-empty field. Empty values are skipped so
// the server applies its own defaults (notably an empty file_id on the first
// chunk is written explicitly by the caller, not here).
func writeOptionalFields(writer *multipart.Writer, fields map[string]string) error {
	for name, value := range fields {
		if value == "" {
			continue
		}
		if err := writer.WriteField(name, value); err != nil {
			return err
		}
	}
	return nil
}

// escapeQuotes mirrors mime/multipart's internal quote escaping for header
// values.
func escapeQuotes(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"':
			b = append(b, '\\', s[i])
		default:
			b = append(b, s[i])
		}
	}
	return string(b)
}
