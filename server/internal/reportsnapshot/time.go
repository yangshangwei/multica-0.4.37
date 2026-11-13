package reportsnapshot

import (
	"errors"
	"fmt"
	"time"
)

const TimeRulesVersion = "resolved-offsets-v1"

var ErrInvalid = errors.New("invalid report snapshot")
var ErrUnsupported = errors.New("unsupported report snapshot version")
var ErrTooLarge = errors.New("report snapshot exceeds size limit")

func timestampValid(t time.Time) bool {
	return !t.IsZero() && t.Year() >= 1 && t.Year() <= 9999 && t.Nanosecond()%1000 == 0
}

// ValidateWindow is for new capture requests. Replay instead checks the saved
// UTC boundaries and offsets without consulting a changed timezone database.
func ValidateWindow(w Window, asOf time.Time) error {
	if w.Timezone == "" || w.Timezone == "Local" {
		return ErrInvalid
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		return fmt.Errorf("%w: timezone", ErrInvalid)
	}
	_, start := w.Start.In(loc).Zone()
	_, end := w.End.In(loc).Zone()
	_, cut := asOf.In(loc).Zone()
	return validateResolvedWindow(w, asOf, TimeBasis{RulesVersion: TimeRulesVersion, ReferenceDate: asOf.In(loc).Format(time.DateOnly), StartOffsetSeconds: start, EndOffsetSeconds: end, AsOfOffsetSeconds: cut})
}

func resolveTimeBasis(in Input) (*TimeBasis, error) {
	if err := ValidateWindow(in.Window, in.AsOf); err != nil {
		return nil, err
	}
	loc, _ := time.LoadLocation(in.Window.Timezone)
	_, start := in.Window.Start.In(loc).Zone()
	_, end := in.Window.End.In(loc).Zone()
	_, cut := in.AsOf.In(loc).Zone()
	out := &TimeBasis{RulesVersion: TimeRulesVersion, ReferenceDate: in.AsOf.In(loc).Format(time.DateOnly), StartOffsetSeconds: start, EndOffsetSeconds: end, AsOfOffsetSeconds: cut}
	if in.Facts.Health != nil {
		if t := in.Facts.Health.LatestUpdateAt; t != nil {
			_, n := t.In(loc).Zone()
			out.LatestUpdateOffsetSeconds = &n
		}
		if t := in.Facts.Health.Project.InProgressSince; t != nil {
			_, n := t.In(loc).Zone()
			out.InProgressSinceOffsetSeconds = &n
		}
	}
	return out, nil
}

func localClock(t time.Time, offset int) time.Time {
	return t.UTC().Add(time.Duration(offset) * time.Second)
}
func offsetValid(n int) bool { return n >= -86400 && n <= 86400 }

func validateResolvedWindow(w Window, asOf time.Time, b TimeBasis) error {
	if b.RulesVersion != TimeRulesVersion {
		return ErrUnsupported
	}
	if !timestampValid(w.Start) || !timestampValid(w.End) || !timestampValid(asOf) || !w.Start.Before(w.End) || w.End.After(asOf) || w.Timezone == "" || w.Timezone == "Local" || len(w.Timezone) > 128 {
		return ErrInvalid
	}
	for _, n := range []int{b.StartOffsetSeconds, b.EndOffsetSeconds, b.AsOfOffsetSeconds} {
		if !offsetValid(n) {
			return ErrInvalid
		}
	}
	if b.ReferenceDate != localClock(asOf, b.AsOfOffsetSeconds).Format(time.DateOnly) {
		return ErrInvalid
	}
	start, end := localClock(w.Start, b.StartOffsetSeconds), localClock(w.End, b.EndOffsetSeconds)
	midnight := func(t time.Time) bool {
		return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
	}
	switch w.PeriodMode {
	case "explicit":
	case "rolling_24h":
		if w.End.Sub(w.Start) != 24*time.Hour {
			return ErrInvalid
		}
	case "rolling_7d":
		if w.End.Sub(w.Start) != 7*24*time.Hour {
			return ErrInvalid
		}
	case "calendar_day":
		if !midnight(start) || !midnight(end) || !start.AddDate(0, 0, 1).Equal(end) {
			return ErrInvalid
		}
	case "calendar_week":
		if !midnight(start) || !midnight(end) || start.Weekday() != time.Monday || !start.AddDate(0, 0, 7).Equal(end) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
