package material

import (
	"strings"
	"testing"
)

func TestMaterialReadinessAndStableSources(t *testing.T) {
	good := RevisionInput{Kind: "requirement", Content: strings.Repeat("A precise requirement. ", 5), Sources: []Source{{ResourceID: "fixture", Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: Reject missing authentication"}, Ready: true}
	if err := ValidateRevision(good); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RevisionInput){"no acceptance": func(v *RevisionInput) { v.AcceptanceCriteria = nil }, "mutable source": func(v *RevisionInput) {
		v.Sources = []Source{{ResourceID: "fixture", Commit: "main", Path: "README.md"}}
	}, "traversal": func(v *RevisionInput) {
		v.Sources = []Source{{ResourceID: "fixture", Commit: strings.Repeat("a", 40), Path: "../secret"}}
	}, "blocked": func(v *RevisionInput) { v.BlockingQuestions = []string{"Who is the user?"} }, "duplicate AC": func(v *RevisionInput) { v.AcceptanceCriteria = []string{"AC1: first", "AC1: second"} }, "plan without tasks": func(v *RevisionInput) { v.Kind = "plan" }} {
		t.Run(name, func(t *testing.T) {
			v := good
			mutate(&v)
			if ValidateRevision(v) == nil {
				t.Fatal("invalid reviewable input accepted")
			}
		})
	}
	draft := RevisionInput{Kind: "requirement", Content: "unfinished"}
	if err := ValidateRevision(draft); err != nil {
		t.Fatal(err)
	}
}
func TestMaterialSettingsDefaultPreservesFalse(t *testing.T) {
	in := SettingsInput{}
	if !in.Settings().AutoStartPlan {
		t.Fatal("default disabled")
	}
	f := false
	in.AutoStartPlan = &f
	if in.Settings().AutoStartPlan {
		t.Fatal("explicit false lost")
	}
	for _, p := range []string{"../x", "a/../x", "/root", "a\\b", ".git/config", "a//b", "a/./b"} {
		if ValidPath(p, false) {
			t.Errorf("unsafe path: %s", p)
		}
	}
}
