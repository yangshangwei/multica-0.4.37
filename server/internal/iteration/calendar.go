package iteration

import (
	"fmt"
	"strings"
	"time"
)

// ParseDate parses a calendar date without converting it through a timezone.
func ParseDate(value string) (time.Time, error) {
	if len(value) != 10 {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	date, err := time.Parse(time.DateOnly, value)
	if err != nil || date.Year() < 1 {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	return date, nil
}

func ValidateTimezone(name string) (*time.Location, error) {
	if name == "" || name == "Local" || strings.TrimSpace(name) != name || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "+") || strings.HasPrefix(name, "-") {
		return nil, fmt.Errorf("invalid IANA timezone %q", name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load IANA timezone: %w", err)
	}
	return loc, nil
}

// DayBoundary returns the first valid instant at or after a local date's
// midnight. A repeated midnight uses the first occurrence; a skipped date uses
// the next existing date. Iterating zone intervals avoids time.Date's ambiguous
// normalization at gaps and folds.
func DayBoundary(date, timezone string) (time.Time, error) {
	day, err := ParseDate(date)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := ValidateTimezone(timezone)
	if err != nil {
		return time.Time{}, err
	}
	// Every IANA offset and date-line transition fits within this search window.
	cursor, limit := day.AddDate(0, 0, -3), day.AddDate(0, 0, 3)
	for cursor.Before(limit) {
		local := cursor.In(loc)
		_, offset := local.Zone()
		_, zoneEnd := local.ZoneBounds()
		candidate := day.Add(-time.Duration(offset) * time.Second)
		if candidate.Before(cursor) {
			candidate = cursor
		}
		if (zoneEnd.IsZero() || candidate.Before(zoneEnd)) && candidate.Before(limit) {
			return candidate.In(loc), nil
		}
		if zoneEnd.IsZero() {
			break
		}
		cursor = zoneEnd
	}
	return time.Time{}, fmt.Errorf("no valid day boundary for %s in %s", date, timezone)
}

type StartDatePreview struct {
	ReferenceDate      string `json:"reference_date"`
	EffectiveStartDate string `json:"effective_start_date"`
	EffectiveEndDate   string `json:"effective_end_date"`
	Timezone           string `json:"timezone"`
}

// StartDates derives the dates that a start confirmation must bind. The caller
// supplies its lock-time sample and compares these facts again on submission.
func StartDates(start, end, timezone, mode string, now time.Time) (StartDatePreview, error) {
	first, err := ParseDate(start)
	if err != nil {
		return StartDatePreview{}, err
	}
	last, err := ParseDate(end)
	if err != nil {
		return StartDatePreview{}, err
	}
	if last.Before(first) {
		return StartDatePreview{}, fmt.Errorf("end date precedes start date")
	}
	loc, err := ValidateTimezone(timezone)
	if err != nil {
		return StartDatePreview{}, err
	}
	today, err := ParseDate(now.In(loc).Format(time.DateOnly))
	if err != nil {
		return StartDatePreview{}, err
	}
	if today.After(last) {
		return StartDatePreview{}, fmt.Errorf("expired iteration must be rescheduled")
	}
	result := StartDatePreview{ReferenceDate: today.Format(time.DateOnly), EffectiveStartDate: start, EffectiveEndDate: end, Timezone: timezone}
	switch mode {
	case "scheduled":
		if today.Before(first) {
			return StartDatePreview{}, fmt.Errorf("future iteration requires today start")
		}
	case "today":
		days := int((last.Unix() - first.Unix()) / 86400)
		result.EffectiveStartDate = today.Format(time.DateOnly)
		result.EffectiveEndDate = today.AddDate(0, 0, days).Format(time.DateOnly)
		if _, err := ParseDate(result.EffectiveEndDate); err != nil {
			return StartDatePreview{}, err
		}
	default:
		return StartDatePreview{}, fmt.Errorf("invalid start mode %q", mode)
	}
	return result, nil
}
