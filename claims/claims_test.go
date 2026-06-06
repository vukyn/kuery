package claims

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestWithPermsRolesIsAdminRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		perms   []string
		roles   []string
		isAdmin bool
	}{
		{
			name:    "member with perms and roles",
			perms:   []string{"user:read", "role:read"},
			roles:   []string{"member"},
			isAdmin: false,
		},
		{
			name:    "admin without perms",
			perms:   nil,
			roles:   []string{"admin"},
			isAdmin: true,
		},
		{
			name:    "empty everything",
			perms:   []string{},
			roles:   []string{},
			isAdmin: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built := NewClaims("user-1", "user@example.com", 3600).
				WithPerms(tt.perms).
				WithRoles(tt.roles).
				WithIsAdmin(tt.isAdmin)

			if got := built.GetPerms(); !slices.Equal(got, tt.perms) {
				t.Errorf("GetPerms() = %v, want %v", got, tt.perms)
			}
			if got := built.GetRoles(); !slices.Equal(got, tt.roles) {
				t.Errorf("GetRoles() = %v, want %v", got, tt.roles)
			}
			if got := built.GetIsAdmin(); got != tt.isAdmin {
				t.Errorf("GetIsAdmin() = %v, want %v", got, tt.isAdmin)
			}
		})
	}
}

func TestHasPerm(t *testing.T) {
	built := NewClaims("user-1", "user@example.com", 3600).
		WithPerms([]string{"user:read", "user:create"})

	tests := []struct {
		name string
		perm string
		want bool
	}{
		{"held permission", "user:read", true},
		{"missing permission", "user:delete", false},
		{"empty permission", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := built.HasPerm(tt.perm); got != tt.want {
				t.Errorf("HasPerm(%q) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}

// TestJSONDecodedClaims simulates MapClaims after a JSON round-trip,
// where arrays become []any instead of []string.
func TestJSONDecodedClaims(t *testing.T) {
	original := NewClaims("user-1", "user@example.com", 3600).
		WithPerms([]string{"user:read", "user:create"}).
		WithRoles([]string{"member", "viewer"}).
		WithIsAdmin(true)

	raw, err := json.Marshal(original.MapClaims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	decoded := Claims{jwt.MapClaims{}}
	if err := json.Unmarshal(raw, &decoded.MapClaims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}

	// confirm we are exercising the []any decode path
	if _, ok := decoded.MapClaims[PermsKey].([]any); !ok {
		t.Fatalf("expected %s to decode as []any, got %T", PermsKey, decoded.MapClaims[PermsKey])
	}

	if got, want := decoded.GetPerms(), []string{"user:read", "user:create"}; !slices.Equal(got, want) {
		t.Errorf("GetPerms() = %v, want %v", got, want)
	}
	if got, want := decoded.GetRoles(), []string{"member", "viewer"}; !slices.Equal(got, want) {
		t.Errorf("GetRoles() = %v, want %v", got, want)
	}
	if !decoded.GetIsAdmin() {
		t.Error("GetIsAdmin() = false, want true")
	}
	if !decoded.HasPerm("user:read") {
		t.Error(`HasPerm("user:read") = false, want true`)
	}
	if decoded.HasPerm("user:delete") {
		t.Error(`HasPerm("user:delete") = true, want false`)
	}
	if got := decoded.GetUserID(); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := decoded.GetEmail(); got != "user@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "user@example.com")
	}
}
