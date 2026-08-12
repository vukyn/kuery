package medioa

import (
	"context"
)

// Limits is the server's published upload limits.
type Limits struct {
	// MaxFileSizeBytes is the per-file ceiling the server enforces — on a
	// single-shot upload's size, and on a chunked upload's assembled total.
	//
	// Zero means the server has no ceiling configured. Treat it as "skip the
	// pre-flight", never as "reject everything": the server publishes its no-cap
	// sentinel as zero rather than as a negative number precisely so a consumer
	// cannot turn it into a signed comparison that fails every file.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes"`
}

// Allows reports whether a file of sizeBytes is within the ceiling. An
// unconfigured ceiling (zero) allows everything, matching the server.
func (l Limits) Allows(sizeBytes int64) bool {
	if l.MaxFileSizeBytes <= 0 {
		return true
	}
	return sizeBytes <= l.MaxFileSizeBytes
}

// Limits fetches the server's upload limits, cached for the configured TTL.
//
// The point of asking is to reject a doomed file locally instead of paying to
// transfer it: the server's 413 names the cap, but only after the upload has
// been spent, and a consumer that hard-coded the number would drift silently the
// moment the server's env changed. So the number is fetched from the side that
// owns it and re-fetched as it ages.
//
// Failures are never cached — a network blip must not pin a consumer to a
// pre-flight it cannot perform for the rest of the TTL. A caller that cannot
// reach this endpoint should proceed with the upload and let the server enforce
// the cap: the pre-flight is an optimisation, and the server's checks are the
// authority.
func (c *Client) Limits(ctx context.Context) (*Limits, error) {
	c.limitsMutex.Lock()
	defer c.limitsMutex.Unlock()

	if c.cachedLimits != nil && c.limitsTTL > 0 && c.now().Sub(c.limitsFetchedAt) < c.limitsTTL {
		// Copy so a caller mutating the result cannot corrupt the cache.
		cached := *c.cachedLimits
		return &cached, nil
	}

	var limits Limits
	if err := c.doGet(ctx, pathLimits, &limits); err != nil {
		return nil, err
	}

	c.cachedLimits = &limits
	c.limitsFetchedAt = c.now()

	fresh := limits
	return &fresh, nil
}
