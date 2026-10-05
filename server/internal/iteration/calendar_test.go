package iteration

import (
	"testing"
	"time"
)

func TestDayBoundaryCalendarTransitions(t *testing.T) {
	tests := []struct {
		zone, date, want string
		duration         time.Duration
	}{
		{"America/New_York", "2026-03-08", "2026-03-08T05:00:00Z", 23 * time.Hour},
		{"America/New_York", "2026-11-01", "2026-11-01T04:00:00Z", 25 * time.Hour},
		{"America/Sao_Paulo", "2018-11-04", "2018-11-04T03:00:00Z", 23 * time.Hour},
		{"America/Havana", "2020-11-01", "2020-11-01T04:00:00Z", 25 * time.Hour},
		{"Pacific/Apia", "2011-12-29", "2011-12-29T10:00:00Z", 24 * time.Hour},
		{"Pacific/Apia", "2011-12-30", "2011-12-30T10:00:00Z", 0},
	}
	for _, tt := range tests {
		t.Run(tt.zone+tt.date, func(t *testing.T) {
			got, err := DayBoundary(tt.date, tt.zone)
			if err != nil || got.UTC().Format(time.RFC3339) != tt.want {
				t.Fatalf("boundary %v %v, want %s", got, err, tt.want)
			}
			date, _ := ParseDate(tt.date)
			next, err := DayBoundary(date.AddDate(0, 0, 1).Format(time.DateOnly), tt.zone)
			if err != nil || next.Sub(got) != tt.duration {
				t.Fatalf("calendar duration %v %v, want %v", next.Sub(got), err, tt.duration)
			}
		})
	}
}

func TestCalendarInputValidation(t *testing.T) {
	for _, zone := range []string{"Local", "+08:00", "UTC+8", "Unknown/Zone", "", "/etc/passwd", "../UTC"} {
		if _, err := ValidateTimezone(zone); err == nil {
			t.Errorf("accepted zone %q", zone)
		}
	}
	for _, date := range []string{"2026-2-01", "2026-02-29", "2026-04-31", "0000-01-01", "2026-01-01T00:00:00Z"} {
		if _, err := ParseDate(date); err == nil {
			t.Errorf("accepted date %q", date)
		}
	}
	if _, err := ParseDate("2024-02-29"); err != nil {
		t.Fatal(err)
	}
}

func TestStartDatesRespectLocalTodayAndCalendarLength(t *testing.T) {
	now := time.Date(2026, 3, 7, 15, 0, 0, 0, time.UTC)
	got, err := StartDates("2026-03-10", "2026-03-23", "America/New_York", "today", now)
	if err != nil || got.ReferenceDate != "2026-03-07" || got.EffectiveStartDate != "2026-03-07" || got.EffectiveEndDate != "2026-03-20" {
		t.Fatalf("start dates: %+v %v", got, err)
	}
	if _, err := StartDates("2026-03-10", "2026-03-23", "UTC", "scheduled", now); err == nil {
		t.Fatal("future scheduled start accepted")
	}
	if _, err := StartDates("2026-03-01", "2026-03-06", "UTC", "today", now); err == nil {
		t.Fatal("expired start accepted")
	}
	if _, err := StartDates("2026-03-10", "2026-03-01", "UTC", "today", now); err == nil {
		t.Fatal("inverted dates accepted")
	}
	before := time.Date(2026, 10, 5, 15, 59, 59, 0, time.UTC)
	a, _ := StartDates("2026-10-10", "2026-10-23", "Asia/Shanghai", "today", before)
	b, _ := StartDates("2026-10-10", "2026-10-23", "Asia/Shanghai", "today", before.Add(time.Second))
	if a.ReferenceDate == b.ReferenceDate || a.EffectiveEndDate == b.EffectiveEndDate {
		t.Fatal("local midnight did not change confirmation facts")
	}
}

func TestStartDatesPreserveLongCalendarSpan(t *testing.T) {
	// Calendar intervals cannot be represented by time.Duration beyond 290 years.
	got, err := StartDates("2400-01-01", "2800-01-01", "UTC", "today", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := ParseDate(got.EffectiveStartDate)
	last, _ := ParseDate(got.EffectiveEndDate)
	scheduledFirst, _ := ParseDate("2400-01-01")
	scheduledLast, _ := ParseDate("2800-01-01")
	if last.Unix()-first.Unix() != scheduledLast.Unix()-scheduledFirst.Unix() {
		t.Fatalf("calendar duration overflowed: %+v", got)
	}
}
