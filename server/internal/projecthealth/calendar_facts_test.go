package projecthealth

import (
	"reflect"
	"testing"
	"time"
)

func TestFrozenCalendarPreservesHealthAndSnapshotVersion(t *testing.T) {
	for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
		t.Run(zone, func(t *testing.T) {
			in := sampleInput()
			in.Timezone = &zone
			in.Now = time.Date(2026, 3, 9, 5, 0, 0, 0, time.UTC)
			update := in.Now.Add(-8 * 24 * time.Hour)
			in.LatestUpdateAt = &update
			in.Project.InProgressSince = &update
			in.Project.Status = "in_progress"
			in.Issues = []Issue{issue("a", "blocked"), issue("b", "done")}
			original, e := Compute(in)
			if e != nil {
				t.Fatal(e)
			}
			loc, e := time.LoadLocation(zone)
			if e != nil {
				t.Fatal(e)
			}
			date := update.In(loc).Format(time.DateOnly)
			frozen, e := ComputeWithCalendar(in, CalendarFacts{ReferenceDate: in.Now.In(loc).Format(time.DateOnly), LatestUpdateDate: &date, InProgressSinceDate: &date})
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(original, frozen) {
				t.Fatalf("health/digest drift: %#v != %#v", original, frozen)
			}
			in.Timezone = str("Historic/UnavailableZone")
			if _, e = ComputeWithCalendar(in, CalendarFacts{ReferenceDate: "2026-03-09", LatestUpdateDate: &date, InProgressSinceDate: &date}); e != nil {
				t.Fatal("frozen calendar loaded current tzdb", e)
			}
		})
	}
}

func TestFrozenCalendarRejectsMissingOrInvalidDates(t *testing.T) {
	in := sampleInput()
	update := in.Now
	in.LatestUpdateAt = &update
	for _, calendar := range []CalendarFacts{{ReferenceDate: "2026-03-10"}, {ReferenceDate: "2026-02-30", LatestUpdateDate: str("2026-03-10")}, {ReferenceDate: "2026-03-10", LatestUpdateDate: str("2026-3-9")}} {
		if _, e := ComputeWithCalendar(in, calendar); e == nil {
			t.Fatal("invalid frozen dates accepted")
		}
	}
}
