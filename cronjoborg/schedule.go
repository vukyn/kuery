package cronjoborg

import (
	"fmt"
	"time"
)

// everyValue is the API's wildcard: a field holding exactly [-1] matches every
// value of that unit.
const everyValue = -1

// every returns a fresh wildcard slice. Each caller gets its own, so a schedule
// handed to the client can never alias another one's field.
func every() []int { return []int{everyValue} }

// Schedules built here always set all five fields, because an omitted field
// defaults to an EMPTY array on the API, and a job with an empty unit never
// runs. Leaving one out is the quiet way to create a job that looks scheduled
// and fires never.

// EveryMinute returns a schedule that runs the job once a minute in timezone
// (an IANA/PHP name such as "UTC" or "Asia/Ho_Chi_Minh"; empty means UTC).
func EveryMinute(timezone string) *Schedule {
	return &Schedule{
		Timezone: timezone,
		Hours:    every(),
		MDays:    every(),
		Minutes:  every(),
		Months:   every(),
		WDays:    every(),
	}
}

// EveryNMinutes returns a schedule that runs the job every n minutes, at
// minutes 0, n, 2n … of every hour.
//
// n must divide 60 evenly (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60). The API
// schedules by an explicit list of minutes, not by an interval, so an n that
// does not divide 60 would leave a short gap at the top of each hour instead of
// the even cadence the caller asked for — this returns an error rather than
// silently producing that.
func EveryNMinutes(n int, timezone string) (*Schedule, error) {
	if n < 1 || n > 60 || 60%n != 0 {
		return nil, fmt.Errorf("cronjoborg: EveryNMinutes needs a divisor of 60 (1..60), got %d", n)
	}

	minutes := make([]int, 0, 60/n)
	for minute := 0; minute < 60; minute += n {
		minutes = append(minutes, minute)
	}

	return &Schedule{
		Timezone: timezone,
		Hours:    every(),
		MDays:    every(),
		Minutes:  minutes,
		Months:   every(),
		WDays:    every(),
	}, nil
}

// MustEveryNMinutes is EveryNMinutes for a constant n, and panics when n is not
// a divisor of 60. Use it only where n is a literal, so the panic can only ever
// fire on the first run after a code change, never on user input.
func MustEveryNMinutes(n int, timezone string) *Schedule {
	schedule, err := EveryNMinutes(n, timezone)
	if err != nil {
		panic(err)
	}
	return schedule
}

// Hourly returns a schedule that runs the job once an hour at the given minute
// (0-59).
func Hourly(minute int, timezone string) (*Schedule, error) {
	if minute < 0 || minute > 59 {
		return nil, fmt.Errorf("cronjoborg: minute must be 0..59, got %d", minute)
	}

	return &Schedule{
		Timezone: timezone,
		Hours:    every(),
		MDays:    every(),
		Minutes:  []int{minute},
		Months:   every(),
		WDays:    every(),
	}, nil
}

// Daily returns a schedule that runs the job once a day at hour:minute in the
// job's own time zone.
func Daily(hour, minute int, timezone string) (*Schedule, error) {
	if hour < 0 || hour > 23 {
		return nil, fmt.Errorf("cronjoborg: hour must be 0..23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return nil, fmt.Errorf("cronjoborg: minute must be 0..59, got %d", minute)
	}

	return &Schedule{
		Timezone: timezone,
		Hours:    []int{hour},
		MDays:    every(),
		Minutes:  []int{minute},
		Months:   every(),
		WDays:    every(),
	}, nil
}

// Weekly returns a schedule that runs the job once a week on weekday at
// hour:minute. time.Weekday and the API agree on the numbering (0 = Sunday).
func Weekly(weekday time.Weekday, hour, minute int, timezone string) (*Schedule, error) {
	if weekday < time.Sunday || weekday > time.Saturday {
		return nil, fmt.Errorf("cronjoborg: weekday must be 0..6, got %d", weekday)
	}

	schedule, err := Daily(hour, minute, timezone)
	if err != nil {
		return nil, err
	}
	schedule.WDays = []int{int(weekday)}
	return schedule, nil
}

// ExpiresAt encodes t as the API's expiry format, a packed decimal
// YYYYMMDDhhmmss read in the job's own time zone — NOT a Unix timestamp. Pass
// a t already expressed in that zone; convert with t.In(loc) first when it is
// not. The zero time encodes as 0, which the API reads as "never expires".
func ExpiresAt(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return int64(t.Year())*1e10 +
		int64(t.Month())*1e8 +
		int64(t.Day())*1e6 +
		int64(t.Hour())*1e4 +
		int64(t.Minute())*1e2 +
		int64(t.Second())
}

// Validate reports whether the schedule's units are inside the ranges the API
// accepts. It does not check the time zone: the valid names are PHP's list, and
// resolving them locally would depend on a tzdata the runtime image may not
// ship — a check that fails on a scratch container and passes on a laptop is
// worse than no check.
func (s Schedule) Validate() error {
	units := []struct {
		name     string
		values   []int
		min, max int
	}{
		{"hours", s.Hours, 0, 23},
		{"mdays", s.MDays, 1, 31},
		{"minutes", s.Minutes, 0, 59},
		{"months", s.Months, 1, 12},
		{"wdays", s.WDays, 0, 6},
	}

	for _, unit := range units {
		if len(unit.values) == 0 {
			return fmt.Errorf("cronjoborg: schedule %s is empty, the job would never run", unit.name)
		}
		for _, value := range unit.values {
			if value == everyValue {
				if len(unit.values) != 1 {
					return fmt.Errorf("cronjoborg: schedule %s mixes -1 with explicit values", unit.name)
				}
				continue
			}
			if value < unit.min || value > unit.max {
				return fmt.Errorf("cronjoborg: schedule %s value %d is outside %d..%d", unit.name, value, unit.min, unit.max)
			}
		}
	}

	if s.ExpiresAt < 0 {
		return fmt.Errorf("cronjoborg: schedule expiresAt must be 0 or YYYYMMDDhhmmss, got %d", s.ExpiresAt)
	}
	return nil
}
