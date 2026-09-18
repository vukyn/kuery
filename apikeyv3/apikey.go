// Package apikey is inbound API-key authentication for Fiber v3: a caller proves
// its identity with a static shared secret sent in a request header.
//
// It is the sibling of the authv3 package rather than a replacement for it. auth
// verifies a signed, expiring token minted for a human; this verifies a long-lived
// key held by a machine — a cron runner, an internal service, a device — where
// there is no login to perform and no token to refresh.
//
// # Only hashes are configured
//
// A service configures the SHA-256 digest of each accepted key, never the key
// itself, so a leaked config file or a leaked environment dump does not hand over
// a working credential. The middleware hashes what the caller presented and
// compares digests.
//
// That is also what makes the constant-time compare meaningful. Two SHA-256
// digests are always 64 characters, so the comparison always runs over the same
// number of bytes. Comparing raw keys instead would return on the first length
// mismatch and leak the secret's length to anyone who can time the endpoint —
// which is the bug this package exists to stop.
//
// # Every key is visited
//
// The match loop has no early exit, so the work done is the same whichever slot
// matched and whether any matched at all. Several keys may be configured at once,
// which is what makes rotation possible: publish the new key, let callers move,
// then drop the old digest.
//
// # It fails closed
//
// An empty Keys slice rejects every request. A deploy that lost its configuration
// must end up with a locked door, not an open one. For the same reason a missing
// header and a wrong key produce exactly the same status and body — the response
// never tells a caller which of the two it got wrong.
//
// # Configuration from an environment variable
//
// ParseKeys turns one envconfig-friendly string into the Keys slice and is the
// validating entry point: it rejects anything that is not a 64-character hex
// digest, so a key pasted into the variable by mistake, or a digest with a typo,
// fails at boot rather than silently never matching.
package apikey

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	pkgCryp "github.com/vukyn/kuery/cryp"
	pkgErr "github.com/vukyn/kuery/http/errors"
	pkgHttp "github.com/vukyn/kuery/http/fiberv3"
)

// Defaults applied when the matching Config field is left empty.
const (
	// DefaultHeader is the header the key is read from when Config.Header is empty.
	DefaultHeader = "X-API-Key"

	// DefaultMessage is the generic rejection message. It is deliberately vague:
	// the same text answers a missing header and a wrong key, so the response
	// cannot be used to tell the two apart.
	DefaultMessage = "invalid api key"

	// DefaultLocalsKey is where the matched Key's Label is stored in Fiber locals
	// when Config.LocalsKey is empty. Label reads this same key back.
	DefaultLocalsKey = "api_key_label"
)

// hashHexLength is the length of a SHA-256 digest in lowercase hex.
const hashHexLength = 64

// Key is one accepted credential.
type Key struct {
	// Hash is the lowercase hex SHA-256 digest of the RAW key. Case is normalised
	// on use, so an upper-case digest still matches.
	Hash string

	// Label is the identity carried into Fiber locals on a match — a client id, a
	// caller name. Optional; empty is fine for a single-caller endpoint, and an
	// empty Label still authenticates (the match itself is the decision, not the
	// presence of a label).
	Label string
}

// Config describes one API-key guard.
type Config struct {
	// Keys are the accepted credentials. EMPTY REJECTS EVERY REQUEST — see the
	// package doc on failing closed. More than one key is the rotation path.
	Keys []Key

	// Header is the request header carrying the raw key. Empty means DefaultHeader.
	Header string

	// Message is the rejection message. Empty means DefaultMessage. Keep it free
	// of any detail about why the request was refused.
	Message string

	// LocalsKey is where the matched Key's Label is stored. Empty means
	// DefaultLocalsKey. Set it only when a service guards two different caller
	// populations on one app and needs to keep their labels apart; then read it
	// back with LabelFrom rather than Label.
	LocalsKey string
}

// New returns a Fiber middleware that accepts a request only when the configured
// header carries a key whose SHA-256 digest is one of cfg.Keys, and stores the
// matched Key's Label in Fiber locals for the handler to read with Label.
//
// Responses:
//   - missing or empty header -> 401
//   - unknown key             -> 401
//   - no keys configured      -> 401
//
// Every rejection goes through pkgErr.Unauthorized so the body is the platform's
// standard 401 envelope, identical to the one the auth middleware returns.
//
// ⚠️ No strings.Clone anywhere in here, and that is correct rather than an
// oversight. Fiber v3 hands out request strings as zero-copy views into fasthttp's
// buffer, which the next request on the connection overwrites, so any value
// RETAINED past the response has to be cloned first (see kuery's CLAUDE.md, and
// ctxv3.GetClientIPFromFiberCtx). This package retains nothing request-derived: the
// header value is hashed into a fresh string and dropped, and the value put into
// locals is the CONFIGURED Label, owned by the caller of New and alive for the
// life of the process. Putting a request-derived string into locals instead would
// reintroduce exactly that bug.
func New(cfg Config) fiber.Handler {
	header := cfg.Header
	if header == "" {
		header = DefaultHeader
	}
	message := cfg.Message
	if message == "" {
		message = DefaultMessage
	}
	localsKey := cfg.LocalsKey
	if localsKey == "" {
		localsKey = DefaultLocalsKey
	}

	// Normalise the configured digests once, at construction, so the per-request
	// path does no string work on the configured side: both operands of the
	// compare are then lowercase hex of the same length. A digest that is not a
	// valid 64-character hex string simply never matches — ParseKeys is where a
	// malformed digest is meant to be caught, loudly, at boot.
	keys := make([]Key, 0, len(cfg.Keys))
	for _, configuredKey := range cfg.Keys {
		keys = append(keys, Key{
			Hash:  strings.ToLower(strings.TrimSpace(configuredKey.Hash)),
			Label: configuredKey.Label,
		})
	}

	return func(c fiber.Ctx) error {
		presentedKey := c.Get(header)
		if presentedKey == "" {
			// Same message and status as a wrong key: a caller must not be able
			// to learn that the header name it guessed was the right one.
			return pkgHttp.Err(c, pkgErr.Unauthorized(message))
		}

		presentedHash := []byte(pkgCryp.HashSHA256(presentedKey))

		// ⚠️ No break and no early return: every configured key is visited on
		// every request, so the time taken cannot reveal WHICH slot matched (or
		// that an early slot did). The hash comparison itself is constant-time
		// for the same reason. Do not "optimise" this loop.
		matched := false
		matchedLabel := ""
		for _, key := range keys {
			if subtle.ConstantTimeCompare(presentedHash, []byte(key.Hash)) == 1 {
				matched = true
				matchedLabel = key.Label
			}
		}

		// Tracked with a bool rather than by testing matchedLabel: Label is
		// optional, so a matching key with no label must still authenticate.
		if !matched {
			return pkgHttp.Err(c, pkgErr.Unauthorized(message))
		}

		c.Locals(localsKey, matchedLabel)
		return c.Next()
	}
}

// Label returns the label of the key the request authenticated with, read from
// DefaultLocalsKey. It is empty when the guard ran with a Key that has no Label,
// and on any request that did not pass through the middleware — so it identifies
// a caller, it never proves one.
func Label(c fiber.Ctx) string {
	return LabelFrom(c, DefaultLocalsKey)
}

// LabelFrom is Label for a guard configured with a custom Config.LocalsKey.
func LabelFrom(c fiber.Ctx, localsKey string) string {
	label, ok := c.Locals(localsKey).(string)
	if !ok {
		return ""
	}
	return label
}

// ParseKeys builds a Keys slice from one configuration string, the shape an
// envconfig/.env variable can carry:
//
//	APIKEY_KEYS="<hash>:<label>,<hash>:<label>"
//	APIKEY_KEYS="<hash>,<hash>"   // labels are optional
//
// Entries are comma-separated, blank entries are skipped, and surrounding spaces
// are trimmed. Each hash is lower-cased and must be exactly 64 hex characters.
// A duplicate hash is an error rather than a silent no-op, because it usually
// means one of the two was meant to be a different key. At most max entries are
// accepted; max <= 0 means no limit.
//
// An empty string parses to an empty slice with no error. That is not an open
// door: New rejects every request when Keys is empty. A service that needs its
// key configuration to be mandatory checks the length itself and refuses to boot.
//
// ⚠️ The returned errors name the POSITION of the bad entry and never its value.
// The most likely reason an entry is not a 64-character hex string is that
// somebody pasted a real key where its digest belonged, and an error message ends
// up in logs.
func ParseKeys(raw string, max int) ([]Key, error) {
	keys := make([]Key, 0)
	seenHashes := make(map[string]struct{})

	for index, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		position := index + 1

		hash := entry
		label := ""
		if separator := strings.Index(entry, ":"); separator >= 0 {
			hash = strings.TrimSpace(entry[:separator])
			label = strings.TrimSpace(entry[separator+1:])
		}
		hash = strings.ToLower(hash)

		if len(hash) != hashHexLength {
			return nil, fmt.Errorf("api key entry %d: hash must be %d hex characters, got %d", position, hashHexLength, len(hash))
		}
		// The error from hex.DecodeString quotes the offending byte, so it is
		// deliberately discarded rather than wrapped.
		if _, err := hex.DecodeString(hash); err != nil {
			return nil, fmt.Errorf("api key entry %d: hash is not hexadecimal", position)
		}
		if _, duplicate := seenHashes[hash]; duplicate {
			return nil, fmt.Errorf("api key entry %d: duplicate hash", position)
		}
		seenHashes[hash] = struct{}{}

		keys = append(keys, Key{Hash: hash, Label: label})
	}

	if max > 0 && len(keys) > max {
		return nil, fmt.Errorf("too many api keys: %d configured, at most %d allowed", len(keys), max)
	}

	return keys, nil
}
