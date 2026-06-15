package memz

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// SetParams describes a Set call. It mirrors memz's cache SetRequest minus the
// server-internal client_id (the server derives the client from the API key).
type SetParams struct {
	// Key is the cache key. Required.
	Key string `json:"key"`
	// Value is the string value to store.
	Value string `json:"value"`
	// NX sets the key only if it does not already exist.
	NX bool `json:"nx"`
	// TTL is the time-to-live in seconds. A value <= 0 stores a permanent entry
	// (the server reports its TTL_NOT_SET sentinel, -1, on subsequent reads).
	TTL int32 `json:"ttl"`
	// KeepTTL preserves the existing entry's remaining TTL on re-set.
	KeepTTL bool `json:"keep_ttl"`
}

// CacheEntry is the value + TTL of a cache key, decoded from a Get or Set
// response. TTL is in seconds; the server reports -1 (TTL_NOT_SET) for a
// permanent entry and -2 (TTL_KEY_NOT_FOUND) for a missing key.
type CacheEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int32  `json:"ttl"`
}

// GetResult is the decoded data of a Get call: the entry plus an existence
// flag (Exist is false when the key is absent).
type GetResult struct {
	CacheEntry
	Exist bool `json:"exist"`
}

// SetResult is the decoded data of a Set call: the stored entry plus the
// success flag (Ok is false when an NX set found the key already present).
type SetResult struct {
	CacheEntry
	Ok bool `json:"ok"`
}

// DelResult is the decoded data of a Del call.
type DelResult struct {
	Ok bool `json:"ok"`
}

// ListItem is one entry in a List result: a key and its TTL (seconds).
type ListItem struct {
	Key string `json:"key"`
	TTL int32  `json:"ttl"`
}

// ListResult is the decoded data of a List call.
type ListResult struct {
	Items []ListItem `json:"items"`
}

// Get fetches the value and TTL of key for the authenticated client. A missing
// key is not an error: the returned GetResult has Exist=false.
func (c *Client) Get(ctx context.Context, key string) (*GetResult, error) {
	if key == "" {
		return nil, errors.New("memz: Get key is required")
	}

	var result GetResult
	if err := c.doJSON(ctx, http.MethodGet, pathCaches+"/"+url.PathEscape(key), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Set stores params.Key holding params.Value, applying TTL / NX / KeepTTL
// semantics, and returns the stored entry and success flag.
func (c *Client) Set(ctx context.Context, params SetParams) (*SetResult, error) {
	if params.Key == "" {
		return nil, errors.New("memz: SetParams.Key is required")
	}

	var result SetResult
	if err := c.doJSON(ctx, http.MethodPost, pathCaches, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Del removes key for the authenticated client.
func (c *Client) Del(ctx context.Context, key string) (*DelResult, error) {
	if key == "" {
		return nil, errors.New("memz: Del key is required")
	}

	var result DelResult
	if err := c.doJSON(ctx, http.MethodDelete, pathCaches+"/"+url.PathEscape(key), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns all keys and their TTLs for the authenticated client.
func (c *Client) List(ctx context.Context) (*ListResult, error) {
	var result ListResult
	if err := c.doJSON(ctx, http.MethodGet, pathCaches, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
