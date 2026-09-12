package service

import (
	"strings"
	"testing"
	"time"
	"unicode"
)

// The autopilot registry is content, and content is what this feature is. These
// tests pin the invariants a release must not break: the roster's shape, the
// cadence and execution mode each template claims, the fact that every embedded
// PROMPT.md is actually readable, and the de-duplication guidance the patrol
// templates depend on to stay quiet.

// TestAutopilotTemplates_ListedRosterIsNine pins the product decision. Four
// general-purpose templates cover the four cadences (hourly, workday, daily,
// weekly) and both execution modes; five specific ones follow for teams that
// already know the need. Order is what the picker renders, so this asserts the
// sequence and not just the count — adding, removing or reordering a listed
// template is a product change and should have to edit this list deliberately.
func TestAutopilotTemplates_ListedRosterIsNine(t *testing.T) {
	want := []string{
		"workday-repo-audit", "release-readiness", "daily-change-review", "hourly-queue-check",
		"stale-pr-reminder", "bug-triage", "weekly-progress-report",
		"dependency-audit", "documentation-check",
	}
	listed := AutopilotTemplates()
	if len(listed) != len(want) {
		keys := make([]string, 0, len(listed))
		for _, template := range listed {
			keys = append(keys, template.Key)
		}
		t.Fatalf("listed roster = %d templates (%s), want %d", len(listed), strings.Join(keys, ", "), len(want))
	}
	for i, key := range want {
		if listed[i].Key != key {
			t.Errorf("listed[%d].Key = %q, want %q (order is product-controlled)", i, listed[i].Key, key)
		}
	}
}

// TestAutopilotTemplates_KeysAreUnique catches the copy-paste that would make
// AutopilotTemplateByKey return the wrong prompt for a key already stamped on
// created autopilot rows.
func TestAutopilotTemplates_KeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, template := range builtinAutopilotTemplates {
		if strings.TrimSpace(template.Key) == "" {
			t.Error("a template has an empty key")
			continue
		}
		if seen[template.Key] {
			t.Errorf("duplicate template key %q", template.Key)
		}
		seen[template.Key] = true
	}
}

// TestAutopilotTemplates_PromptsAreEmbedded proves each PROMPT.md ships in the
// binary and is readable. Prompt() swallows the read error on purpose, so without
// this an unreadable file would surface as an autopilot created with no prompt.
func TestAutopilotTemplates_PromptsAreEmbedded(t *testing.T) {
	for _, template := range builtinAutopilotTemplates {
		prompt := template.Prompt()
		if strings.TrimSpace(prompt) == "" {
			t.Errorf("%s: PROMPT.md is empty or unreadable", template.Key)
			continue
		}
		if strings.Contains(prompt, "{{") {
			t.Errorf("%s: prompt contains an unsubstituted placeholder; template prompts are copied verbatim", template.Key)
		}
	}
}

// The preview and dispatch share these bodies, independently of picker locale.
func TestAutopilotTemplates_PromptsUseCanonicalChinese(t *testing.T) {
	for _, template := range builtinAutopilotTemplates {
		t.Run(template.Key, func(t *testing.T) {
			prompt := template.Prompt()
			if !strings.HasPrefix(prompt, "# "+template.Title("zh")+"\n") {
				t.Errorf("prompt heading must match the Chinese catalog title %q", template.Title("zh"))
			}
			if !strings.Contains(prompt, "中文") {
				t.Error("prompt must state the team's Chinese output language")
			}
			for _, line := range strings.Split(prompt, "\n") {
				if strings.HasPrefix(line, "#") && !strings.ContainsFunc(line, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
					t.Errorf("prompt contains an untranslated heading: %q", line)
				}
			}
		})
	}
}

func TestAutopilotTemplates_SummariesUseThePrecreatedIssue(t *testing.T) {
	for _, template := range builtinAutopilotTemplates {
		if template.ExecutionMode != "create_issue" {
			continue
		}
		if !strings.Contains(template.Prompt(), "本次汇总任务已由系统创建") || !strings.Contains(template.Prompt(), "评论") {
			t.Errorf("%s: summary must deliver a comment on the issue dispatch already created", template.Key)
		}
	}
}

func TestAutopilotTemplate_BugTriageUsesValidPriorities(t *testing.T) {
	template, ok := AutopilotTemplateByKey("bug-triage")
	if !ok {
		t.Fatal("bug-triage template missing")
	}
	for _, value := range []string{"urgent", "high", "medium", "low", "none", "contributor", "observer"} {
		if !strings.Contains(template.Prompt(), "`"+value+"`") {
			t.Errorf("bug-triage prompt must describe the protocol value %q explicitly", value)
		}
	}
	if template.Version != 2 {
		t.Errorf("bug-triage version = %d, want 2 for the corrected severity-to-priority mapping", template.Version)
	}
}

// TestAutopilotTemplates_ExecutionModes pins which templates are summaries and
// which are patrols. A patrol flipped to create_issue would file an issue on every
// tick — 24 a day for the hourly one — which is the failure this split exists to
// prevent.
func TestAutopilotTemplates_ExecutionModes(t *testing.T) {
	want := map[string]string{
		"workday-repo-audit":     "run_only",
		"release-readiness":      "create_issue",
		"daily-change-review":    "create_issue",
		"hourly-queue-check":     "run_only",
		"stale-pr-reminder":      "run_only",
		"bug-triage":             "create_issue",
		"weekly-progress-report": "create_issue",
		"dependency-audit":       "run_only",
		"documentation-check":    "run_only",
	}
	for key, mode := range want {
		template, ok := AutopilotTemplateByKey(key)
		if !ok {
			t.Fatalf("template %q is missing from the roster", key)
		}
		if template.ExecutionMode != mode {
			t.Errorf("%s execution_mode = %q, want %q", key, template.ExecutionMode, mode)
		}
	}
	for _, template := range builtinAutopilotTemplates {
		if template.ExecutionMode != "create_issue" && template.ExecutionMode != "run_only" {
			t.Errorf("%s execution_mode = %q, want create_issue or run_only", template.Key, template.ExecutionMode)
		}
		// A template added without a line above would otherwise get its mode
		// checked only for being one of the two legal values, which is the weaker
		// half of this test.
		if _, pinned := want[template.Key]; !pinned {
			t.Errorf("%s has no pinned execution mode in this test", template.Key)
		}
	}
}

// TestAutopilotTemplates_CronExpressionsAreValid runs every template's cron
// through the same evaluator the trigger write path uses, and pins the cadence
// each one claims against a fixed anchor. A typo here would only surface as a 400
// when someone tried to create from the template.
func TestAutopilotTemplates_CronExpressionsAreValid(t *testing.T) {
	// 2026-06-21 is a Sunday, so the workday and Monday expressions both have to
	// skip forward to Monday the 22nd.
	anchor := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	want := map[string]time.Time{
		"workday-repo-audit":     time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
		"release-readiness":      time.Date(2026, 6, 22, 17, 0, 0, 0, time.UTC),
		"daily-change-review":    time.Date(2026, 6, 21, 18, 0, 0, 0, time.UTC),
		"hourly-queue-check":     time.Date(2026, 6, 21, 1, 0, 0, 0, time.UTC),
		"stale-pr-reminder":      time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC),
		"bug-triage":             time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
		"weekly-progress-report": time.Date(2026, 6, 22, 17, 0, 0, 0, time.UTC),
		"dependency-audit":       time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC),
		"documentation-check":    time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC),
	}
	for _, template := range builtinAutopilotTemplates {
		// The trigger create path calls ComputeNextRun; a cron this rejects cannot
		// be turned into a schedule trigger at all.
		if _, err := ComputeNextRun(template.CronExpression, "UTC"); err != nil {
			t.Errorf("%s cron %q rejected by ComputeNextRun: %v", template.Key, template.CronExpression, err)
			continue
		}
		expected, pinned := want[template.Key]
		if !pinned {
			t.Errorf("%s has no pinned cadence in this test", template.Key)
			continue
		}
		got, err := NextOccurrenceAfterUTC(template.CronExpression, "UTC", anchor)
		if err != nil {
			t.Errorf("%s cron %q: %v", template.Key, template.CronExpression, err)
			continue
		}
		if !got.Equal(expected) {
			t.Errorf("%s cron %q next run after %s = %s, want %s",
				template.Key, template.CronExpression,
				anchor.Format(time.RFC3339), got.Format(time.RFC3339), expected.Format(time.RFC3339))
		}
	}
}

// TestAutopilotTemplates_PatrolPromptsCarryDeduplicationGuidance is the reason the
// run_only templates are safe to run hourly. There is no system-level cross-period
// de-duplication (only a 60s debounce), so the prompt is the whole mitigation: check
// what this autopilot already filed, comment instead of re-filing, and stay silent
// when there is nothing to report.
func TestAutopilotTemplates_PatrolPromptsCarryDeduplicationGuidance(t *testing.T) {
	required := []string{
		"## 创建任务前",
		"尚未关闭的任务",
		"补充评论",
		"不创建任务，也不发表评论",
	}
	wantPatrols := map[string]bool{
		"workday-repo-audit":  true,
		"hourly-queue-check":  true,
		"stale-pr-reminder":   true,
		"dependency-audit":    true,
		"documentation-check": true,
	}
	patrols := map[string]bool{}
	for _, template := range builtinAutopilotTemplates {
		if template.ExecutionMode != "run_only" {
			continue
		}
		patrols[template.Key] = true
		prompt := template.Prompt()
		for _, phrase := range required {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("%s: run_only prompt is missing %q; without it the patrol files a duplicate issue every tick", template.Key, phrase)
			}
		}
		// A patrol has no pre-created issue to write into, so a prompt telling the
		// agent to comment on "this issue" points at nothing. That wording is
		// correct only in create_issue mode, where dispatch made the issue first.
		if strings.Contains(prompt, "on this issue") || strings.Contains(prompt, "本次汇总任务已由系统创建") {
			t.Errorf("%s: run_only prompt says to post \"on this issue\", but run_only pre-creates no issue for it to post on", template.Key)
		}
	}
	for key := range wantPatrols {
		if !patrols[key] {
			t.Errorf("%s is no longer run_only; a patrol flipped to create_issue files an issue on every tick", key)
		}
	}
	for key := range patrols {
		if !wantPatrols[key] {
			t.Errorf("%s is run_only but is not a pinned patrol in this test", key)
		}
	}
}

// TestAutopilotTemplates_IssueTitleTemplates pins how a create_issue run's
// issues stay tellable apart. Without a title template the created autopilot
// falls back to its own title at dispatch, so a daily summary files thirty
// identically-named issues in a month. Only create_issue templates carry one —
// run_only never creates the issue itself, so there is no title to template.
func TestAutopilotTemplates_IssueTitleTemplates(t *testing.T) {
	for _, template := range builtinAutopilotTemplates {
		if template.ExecutionMode == "create_issue" {
			if want := template.Title("zh") + " — {{date}}"; template.IssueTitleTemplate != want {
				t.Errorf("%s: issue title template = %q, want canonical Chinese title %q", template.Key, template.IssueTitleTemplate, want)
			}
			if template.IssueTitleTemplate == "" {
				t.Errorf("%s: create_issue template has no issue title template; every run would file an issue named after the autopilot itself", template.Key)
				continue
			}
			if !strings.Contains(template.IssueTitleTemplate, "{{date}}") {
				t.Errorf("%s: issue title template %q carries no {{date}}; one issue per period needs the period in the title", template.Key, template.IssueTitleTemplate)
			}
			// The from-template endpoint copies this verbatim; a template that
			// fails validation would surface as a 500 on a later edit, not at
			// the registry where it belongs.
			if err := ValidateIssueTitleTemplate(template.IssueTitleTemplate); err != nil {
				t.Errorf("%s: issue title template %q does not validate: %v", template.Key, template.IssueTitleTemplate, err)
			}
		} else if template.ExecutionMode == "run_only" {
			if template.IssueTitleTemplate != "" {
				t.Errorf("%s: run_only template sets issue title template %q, but run_only never creates an issue to title", template.Key, template.IssueTitleTemplate)
			}
		}
	}
}

// TestAutopilotTemplates_LocalizedCopyIsComplete keeps the picker from showing an
// English label inside an otherwise translated screen.
func TestAutopilotTemplates_LocalizedCopyIsComplete(t *testing.T) {
	for _, template := range builtinAutopilotTemplates {
		for _, language := range TemplateLanguages {
			if strings.TrimSpace(template.Titles[language]) == "" {
				t.Errorf("%s has no %s title", template.Key, language)
			}
			if strings.TrimSpace(template.Descriptions[language]) == "" {
				t.Errorf("%s has no %s description", template.Key, language)
			}
			if strings.TrimSpace(template.Categories[language]) == "" {
				t.Errorf("%s has no %s category label", template.Key, language)
			}
		}
		if strings.TrimSpace(template.Category) == "" {
			t.Errorf("%s has no stable category slug", template.Key)
		}
		if strings.TrimSpace(template.AvatarEmoji) == "" {
			t.Errorf("%s has no avatar emoji", template.Key)
		}
		if template.Version < 1 {
			t.Errorf("%s version = %d, want >= 1", template.Key, template.Version)
		}
	}
}

// TestAutopilotTemplate_FallsBackToEnglish pins the unsupported-locale behaviour
// the API relies on instead of returning a 400 for picker copy.
func TestAutopilotTemplate_FallsBackToEnglish(t *testing.T) {
	template, ok := AutopilotTemplateByKey("hourly-queue-check")
	if !ok {
		t.Fatal("hourly-queue-check template missing")
	}
	if got := template.Title("de"); got != template.Titles["en"] {
		t.Errorf("Title(\"de\") = %q, want the English title %q", got, template.Titles["en"])
	}
	if got := template.Description("de"); got != template.Descriptions["en"] {
		t.Errorf("Description(\"de\") = %q, want the English description %q", got, template.Descriptions["en"])
	}
	if got := template.CategoryLabel("de"); got != template.Categories["en"] {
		t.Errorf("CategoryLabel(\"de\") = %q, want the English label %q", got, template.Categories["en"])
	}
	if got := template.Title("zh"); got != template.Titles["zh"] {
		t.Errorf("Title(\"zh\") = %q, want %q", got, template.Titles["zh"])
	}
}

// TestAutopilotTemplateByKey_RejectsUnknown covers the validation path the
// from-template endpoint uses to turn a bad key into a 400 rather than a 500.
//
// The keys probed here are the ids the six templates carried while they were
// hardcoded in the frontend. Five were renamed to the hyphenated scheme on the way
// into this registry and `daily_news` has no successor at all — the web-search
// digest was dropped so the roster stays adoptable without internet access.
// Nothing ever persisted the old form — no autopilot row was ever stamped with
// one — so all six must stay unknown rather than quietly resolving to anything.
func TestAutopilotTemplateByKey_RejectsUnknown(t *testing.T) {
	legacyIDs := []string{
		"daily_news", "pr_review", "bug_triage",
		"weekly_progress", "dependency_audit", "documentation_check",
	}
	for _, key := range legacyIDs {
		if _, ok := AutopilotTemplateByKey(key); ok {
			t.Errorf("AutopilotTemplateByKey accepted the pre-registry id %q, which is not a roster key", key)
		}
	}
	if _, ok := AutopilotTemplateByKey(""); ok {
		t.Error("AutopilotTemplateByKey accepted an empty key")
	}
}
