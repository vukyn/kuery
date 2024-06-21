package conv

import (
	"time"

	"github.com/vukyn/kuery/t"
)

func FormatUnixToString(d int64, format t.DateTime) string {
	var (
		UNIX_MILLI int64 = 1000000000000
	)
	if d <= UNIX_MILLI {
		d *= 1000
	}

	switch format {
	case t.RFC822:
		return time.UnixMilli(d).Format(time.RFC822)
	case t.Kitchen:
		return time.UnixMilli(d).Format(time.Kitchen)
	case t.UnixDate:
		return time.UnixMilli(d).Format(time.UnixDate)
	case t.HH_MM_SS:
		return time.UnixMilli(d).Format(time.TimeOnly)
	case t.YYYY_MM_DD:
		return time.UnixMilli(d).Format(time.DateOnly)
	case t.DD_MM_YYYY:
		return time.UnixMilli(d).Format(string(t.DD_MM_YYYY))
	case t.YYYY_MM_DD_HH_MM_SS:
		return time.UnixMilli(d).Format(time.DateTime)
	case t.DD_MM_YYYY_HH_MM_SS:
		return time.UnixMilli(d).Format(string(t.DD_MM_YYYY_HH_MM_SS))
	default:
		return time.UnixMilli(d).Format(string(format))
	}
}
