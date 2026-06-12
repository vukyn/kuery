package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/robfig/cron/v3"
)

// scheduleKind enumerates the schedule modes the engine supports.
type scheduleKind int

const (
	// kindCron is a raw 5-field standard cron expression (isme).
	kindCron scheduleKind = iota
	// kindDaily fires once a day at a fixed time (medioa2 daily preset).
	kindDaily
	// kindWeekly fires once a week on a fixed weekday at a fixed time
	// (medioa2 weekly preset).
	kindWeekly
)

// Schedule is an opaque schedule value shared by both schedule modes (raw cron
// and friendly daily/weekly preset). Construct one with Cron, Daily, Weekly, or
// FromSpec; the engine turns it into a gocron job definition internally. The
// zero value is not a valid schedule.
type Schedule struct {
	kind scheduleKind
	// cron holds the raw expression for kindCron.
	cron string
	// hour/min hold the fire time (UTC or engine location) for daily/weekly.
	hour uint
	min  uint
	// dow holds the weekday for kindWeekly.
	dow time.Weekday
}

// Cron builds a raw-cron schedule from a 5-field standard cron expression. This
// is isme's mode; the expression is passed through to gocron unchanged (seconds
// disabled, matching the original engine).
func Cron(expr string) Schedule {
	return Schedule{kind: kindCron, cron: expr}
}

// Daily builds a schedule that fires once a day at runAt (HH:MM, 24-hour). It
// panics only via FromSpec/ValidateSpec misuse is avoided — callers that take
// untrusted input should validate with ValidateSpec or use FromSpec, which
// returns an error instead. A malformed runAt here yields a schedule whose
// jobDefinition fails at install time.
func Daily(runAt string) Schedule {
	hour, min, _ := parseHHMM(runAt)
	return Schedule{kind: kindDaily, hour: hour, min: min}
}

// Weekly builds a schedule that fires once a week on dow at runAt (HH:MM,
// 24-hour). See Daily regarding validation of untrusted runAt input.
func Weekly(dow time.Weekday, runAt string) Schedule {
	hour, min, _ := parseHHMM(runAt)
	return Schedule{kind: kindWeekly, hour: hour, min: min, dow: dow}
}

// jobDefinition maps the schedule to a gocron job definition. It returns an
// error for an unconstructable schedule (empty cron expression or an unknown
// kind), so the engine can fail a single job's install without aborting others.
func (s Schedule) jobDefinition() (gocron.JobDefinition, error) {
	switch s.kind {
	case kindCron:
		if strings.TrimSpace(s.cron) == "" {
			return nil, fmt.Errorf("scheduler: empty cron expression")
		}
		return gocron.CronJob(s.cron, false), nil
	case kindDaily:
		return gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(s.hour, s.min, 0))), nil
	case kindWeekly:
		return gocron.WeeklyJob(1, gocron.NewWeekdays(s.dow), gocron.NewAtTimes(gocron.NewAtTime(s.hour, s.min, 0))), nil
	default:
		return nil, fmt.Errorf("scheduler: unknown schedule kind %d", s.kind)
	}
}

// ValidateCron reports whether expr is a valid 5-field standard cron
// expression. It uses robfig/cron's standard parser (no seconds field), which
// matches the semantics of gocron.CronJob(expr, false).
func ValidateCron(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return fmt.Errorf("scheduler: empty cron expression")
	}
	if _, err := cron.ParseStandard(expr); err != nil {
		return fmt.Errorf("scheduler: invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// ValidateSpec validates a friendly schedule spec: frequency must be "daily" or
// "weekly"; runAt must be HH:MM in 00:00–23:59; for "weekly", dayOfWeek must be
// 0–6 (Sunday–Saturday). dayOfWeek is ignored for "daily".
func ValidateSpec(frequency, runAt string, dayOfWeek int) error {
	switch frequency {
	case "daily":
		// dayOfWeek ignored.
	case "weekly":
		if dayOfWeek < 0 || dayOfWeek > 6 {
			return fmt.Errorf("scheduler: day_of_week must be 0-6, got %d", dayOfWeek)
		}
	default:
		return fmt.Errorf("scheduler: frequency must be daily or weekly, got %q", frequency)
	}
	if _, _, err := parseHHMM(runAt); err != nil {
		return err
	}
	return nil
}

// FromSpec validates a friendly schedule spec and builds the corresponding
// Schedule. "daily" ignores dayOfWeek; "weekly" uses it as the time.Weekday. An
// invalid spec returns a non-nil error and the zero Schedule.
func FromSpec(frequency, runAt string, dayOfWeek int) (Schedule, error) {
	if err := ValidateSpec(frequency, runAt, dayOfWeek); err != nil {
		return Schedule{}, err
	}
	switch frequency {
	case "daily":
		return Daily(runAt), nil
	case "weekly":
		return Weekly(time.Weekday(dayOfWeek), runAt), nil
	default:
		// Unreachable: ValidateSpec rejects other frequencies.
		return Schedule{}, fmt.Errorf("scheduler: frequency must be daily or weekly, got %q", frequency)
	}
}

// parseHHMM parses an "HH:MM" 24-hour time string into hour (0-23) and minute
// (0-59). gocron.NewAtTime takes uint arguments, so the returned values are
// uint.
func parseHHMM(s string) (hour, min uint, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("scheduler: run_at must be HH:MM, got %q", s)
	}
	h, herr := strconv.Atoi(parts[0])
	m, merr := strconv.Atoi(parts[1])
	if herr != nil || merr != nil {
		return 0, 0, fmt.Errorf("scheduler: run_at must be HH:MM, got %q", s)
	}
	if h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("scheduler: run_at hour must be 00-23, got %q", s)
	}
	if m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("scheduler: run_at minute must be 00-59, got %q", s)
	}
	return uint(h), uint(m), nil
}
