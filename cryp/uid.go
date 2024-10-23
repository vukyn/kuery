package cryp

import (
	uuid "github.com/google/uuid"
	ulid "github.com/oklog/ulid/v2"
)

// UUID generate a UUID
//
// Example:
//
//	UUID() => "f47ac10b-58cc-4372-a567-0e02b2c3d479"
func UUID() string {
	return uuid.New().String()
}

// ULID generate a ULID
//
// Example:
//
//	ULID() => "01D7Z9Z1ZQ0QZQZQZQZQZQZQZQ"
func ULID() string {
	return ulid.Make().String()
}
