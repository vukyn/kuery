package scheduler

import (
	"testing"
	"time"
)

func TestScheduleJobDefinition(t *testing.T) {
	tests := []struct {
		name     string
		schedule Schedule
		wantErr  bool
	}{
		{"valid cron", Cron("0 4 * * *"), false},
		{"empty cron", Cron(""), true},
		{"whitespace cron", Cron("   "), true},
		{"valid daily", Daily("04:00"), false},
		{"valid weekly", Weekly(time.Sunday, "23:30"), false},
		{"unknown kind", Schedule{kind: scheduleKind(99)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := tt.schedule.jobDefinition()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (def=%v)", def)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if def == nil {
				t.Fatal("expected non-nil job definition")
			}
		})
	}
}

func TestValidateCron(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"valid 5-field", "0 4 * * *", false},
		{"valid every minute", "* * * * *", false},
		{"valid ranges", "0 0 1 1 0", false},
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"garbage", "not a cron", true},
		{"too many fields (6-field/seconds)", "0 0 4 * * *", true},
		{"out of range", "99 4 * * *", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCron(tt.expr)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name      string
		frequency string
		runAt     string
		dayOfWeek int
		wantErr   bool
	}{
		{"daily ok", "daily", "04:00", 0, false},
		{"daily ignores dow", "daily", "04:00", 99, false},
		{"weekly ok sunday", "weekly", "23:30", 0, false},
		{"weekly ok saturday", "weekly", "00:00", 6, false},
		{"bad frequency", "monthly", "04:00", 0, true},
		{"empty frequency", "", "04:00", 0, true},
		{"weekly dow too low", "weekly", "04:00", -1, true},
		{"weekly dow too high", "weekly", "04:00", 7, true},
		{"bad time format", "daily", "4am", 0, true},
		{"bad time no colon", "daily", "0400", 0, true},
		{"hour out of range", "daily", "24:00", 0, true},
		{"minute out of range", "daily", "04:60", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpec(tt.frequency, tt.runAt, tt.dayOfWeek)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFromSpec(t *testing.T) {
	tests := []struct {
		name      string
		frequency string
		runAt     string
		dayOfWeek int
		wantErr   bool
		wantKind  scheduleKind
		wantHour  uint
		wantMin   uint
		wantDow   time.Weekday
	}{
		{
			name: "daily ignores dow", frequency: "daily", runAt: "04:15", dayOfWeek: 5,
			wantKind: kindDaily, wantHour: 4, wantMin: 15,
		},
		{
			name: "weekly uses dow", frequency: "weekly", runAt: "23:45", dayOfWeek: 3,
			wantKind: kindWeekly, wantHour: 23, wantMin: 45, wantDow: time.Wednesday,
		},
		{name: "invalid frequency", frequency: "yearly", runAt: "04:00", dayOfWeek: 0, wantErr: true},
		{name: "invalid time", frequency: "daily", runAt: "99:99", dayOfWeek: 0, wantErr: true},
		{name: "invalid weekly dow", frequency: "weekly", runAt: "04:00", dayOfWeek: 9, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := FromSpec(tt.frequency, tt.runAt, tt.dayOfWeek)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if schedule.kind != tt.wantKind {
				t.Errorf("kind = %d, want %d", schedule.kind, tt.wantKind)
			}
			if schedule.hour != tt.wantHour {
				t.Errorf("hour = %d, want %d", schedule.hour, tt.wantHour)
			}
			if schedule.min != tt.wantMin {
				t.Errorf("min = %d, want %d", schedule.min, tt.wantMin)
			}
			if tt.wantKind == kindWeekly && schedule.dow != tt.wantDow {
				t.Errorf("dow = %v, want %v", schedule.dow, tt.wantDow)
			}
			// Constructed schedule must produce a usable gocron definition.
			if _, err := schedule.jobDefinition(); err != nil {
				t.Errorf("jobDefinition failed for valid spec: %v", err)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantErr  bool
		wantHour uint
		wantMin  uint
	}{
		{"midnight", "00:00", false, 0, 0},
		{"end of day", "23:59", false, 23, 59},
		{"single-digit padded", "04:05", false, 4, 5},
		{"missing colon", "0400", true, 0, 0},
		{"too many parts", "04:00:00", true, 0, 0},
		{"non-numeric hour", "aa:00", true, 0, 0},
		{"non-numeric minute", "04:bb", true, 0, 0},
		{"hour 24", "24:00", true, 0, 0},
		{"minute 60", "04:60", true, 0, 0},
		{"empty", "", true, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, min, err := parseHHMM(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hour != tt.wantHour || min != tt.wantMin {
				t.Errorf("got (%d, %d), want (%d, %d)", hour, min, tt.wantHour, tt.wantMin)
			}
		})
	}
}
