package cronjoborg

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// listFoldersResponse is the raw shape of GET /folders.
type listFoldersResponse struct {
	Folders []Folder `json:"folders"`
}

// folderDetailsResponse is the raw shape of GET /folders/{folderId}.
type folderDetailsResponse struct {
	FolderDetails Folder `json:"folderDetails"`
}

// createFolderResponse is the raw shape of PUT /folders.
type createFolderResponse struct {
	FolderID int64 `json:"folderId"`
}

// folderEnvelope wraps a folder payload as {"folder": {…}}.
type folderEnvelope struct {
	Folder folderInput `json:"folder"`
}

// folderInput is the write shape of a folder. Only the title is settable.
type folderInput struct {
	Title string `json:"title"`
}

// ListFolders returns every folder in the account.
//
// Rate limit: 5 requests per second.
func (c *Client) ListFolders(ctx context.Context) ([]Folder, error) {
	var response listFoldersResponse
	if err := c.doJSON(ctx, http.MethodGet, pathFolders, nil, &response); err != nil {
		return nil, err
	}
	return response.Folders, nil
}

// GetFolder returns one folder.
//
// Rate limit: 5 requests per second.
func (c *Client) GetFolder(ctx context.Context, folderID int64) (*Folder, error) {
	if folderID <= 0 {
		return nil, errors.New("cronjoborg: folderID is required")
	}

	var response folderDetailsResponse
	if err := c.doJSON(ctx, http.MethodGet, folderPath(folderID), nil, &response); err != nil {
		return nil, err
	}
	return &response.FolderDetails, nil
}

// CreateFolder creates a folder and returns its identifier. Folder titles are
// unique per account: creating one that already exists fails with ErrConflict,
// which is the signal to look the existing folder up rather than an error to
// report.
//
// Rate limit: 1 request per second and 10 requests per minute.
func (c *Client) CreateFolder(ctx context.Context, title string) (int64, error) {
	if strings.TrimSpace(title) == "" {
		return 0, errors.New("cronjoborg: folder title is required")
	}

	var response createFolderResponse
	body := folderEnvelope{Folder: folderInput{Title: title}}
	if err := c.doJSON(ctx, http.MethodPut, pathFolders, body, &response); err != nil {
		return 0, err
	}
	return response.FolderID, nil
}

// UpdateFolder renames a folder. Titles are unique per account, so a name
// already in use fails with ErrConflict.
//
// Rate limit: 1 request per second.
func (c *Client) UpdateFolder(ctx context.Context, folderID int64, title string) error {
	if folderID <= 0 {
		return errors.New("cronjoborg: folderID is required")
	}
	if strings.TrimSpace(title) == "" {
		return errors.New("cronjoborg: folder title is required")
	}

	body := folderEnvelope{Folder: folderInput{Title: title}}
	return c.doJSON(ctx, http.MethodPatch, folderPath(folderID), body, nil)
}

// DeleteFolder removes a folder. The jobs it contains are NOT deleted: the API
// moves them to the root folder (folderId 0) first.
//
// Rate limit: 1 request per second.
func (c *Client) DeleteFolder(ctx context.Context, folderID int64) error {
	if folderID <= 0 {
		return errors.New("cronjoborg: folderID is required")
	}
	return c.doJSON(ctx, http.MethodDelete, folderPath(folderID), nil, nil)
}

// folderPath builds /folders/{folderId}.
func folderPath(folderID int64) string {
	return pathFolders + "/" + strconv.FormatInt(folderID, 10)
}
