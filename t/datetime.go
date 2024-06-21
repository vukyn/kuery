package t

import "time"

type DateTime string

const (
	RFC822              DateTime = time.RFC822
	Kitchen             DateTime = time.Kitchen
	UnixDate            DateTime = time.UnixDate
	HH_MM_SS            DateTime = time.TimeOnly
	YYYY_MM_DD          DateTime = time.DateOnly
	DD_MM_YYYY          DateTime = "02-01-2006"
	YYYY_MM_DD_HH_MM_SS DateTime = time.DateTime
	DD_MM_YYYY_HH_MM_SS DateTime = "02-01-2006 15:04:05"
)
