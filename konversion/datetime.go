package konversion

import "time"

type DateTime string

const (
	RFC822 DateTime = time.RFC822
	Kitchen DateTime = time.Kitchen
	UnixDate DateTime = time.UnixDate
	YYYY_MM_DD DateTime = "2006-01-02"
	DD_MM_YYYY DateTime = "02-01-2006"
	YYYY_MM_DD_HH_MM_SS DateTime = "2006-01-02 15:04:05"
	DD_MM_YYYY_HH_MM_SS DateTime = "02-01-2006 15:04:05"
)

func FormatUnixToString(d int, format DateTime) interface{} {
	if d != 0 {
		switch format {
		case RFC822:
			return time.UnixMilli(int64(d)).Format(time.RFC822)
		case Kitchen:
			return time.UnixMilli(int64(d)).Format(time.Kitchen)
		case UnixDate:
			return time.UnixMilli(int64(d)).Format(time.UnixDate)
		case YYYY_MM_DD:
			return time.UnixMilli(int64(d)).Format(string(YYYY_MM_DD))
		case DD_MM_YYYY:
			return time.UnixMilli(int64(d)).Format(string(DD_MM_YYYY))
		case YYYY_MM_DD_HH_MM_SS:
			return time.UnixMilli(int64(d)).Format(string(YYYY_MM_DD_HH_MM_SS))
		case DD_MM_YYYY_HH_MM_SS:
			return time.UnixMilli(int64(d)).Format(string(DD_MM_YYYY_HH_MM_SS))
		default:
			return time.UnixMilli(int64(d)).Format(string(YYYY_MM_DD_HH_MM_SS))
		}
	}
	return nil
}
