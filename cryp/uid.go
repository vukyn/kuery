package cryp

import (
	uuid "github.com/google/uuid"
	ulid "github.com/oklog/ulid/v2"
)

func UUID() string {
	return uuid.New().String()
}

func ULID() string {
	return ulid.Make().String()
}
