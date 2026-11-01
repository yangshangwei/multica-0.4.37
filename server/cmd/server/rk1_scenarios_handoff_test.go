//go:build agentintegration && !windows

package main

import (
	"regexp"
	"strings"
)

// Scenario text and answer formats are frozen here. A change is a framework
// change: it moves the framework SHA-256 and voids the registration.
const rk1NoWrites = " Do not modify files, accounts or collaboration memory, do not post comments, and do not start background work."

func rk1E0a(f *rk1Fixture, tr *rk1Trial, m rk1Mode) {
	f.agentTarget("agent")
	f.submit(f.owner, "agent", "Use only the supplied `multica issue get` command to read this test issue. Then reply with exactly HOST_SMOKE_OK and no other text."+rk1NoWrites)
	f.control(false)
	o := f.run(tr, "measured", true, f.nextRun(), f.plan(m, "smoke", map[string]string{"answer": "HOST_SMOKE_OK", "wrong_answer": "HOST_SMOKE_FAIL"}), false, nil)
	if o == nil {
		return
	}
	if !o.cliSeen(f.issue, "context") {
		f.incomplete(tr, "unexposed:no_cli_context_read")
		return
	}
	verdict := "fail"
	if o.answer() == "HOST_SMOKE_OK" {
		verdict = "pass"
	}
	f.finish(tr, o, verdict, map[string]any{"exact_body": o.answer() == "HOST_SMOKE_OK", "cli_context_read": true,
		"checkpoints": o.Events["record_checkpoint"], "seal_reason": o.SealReason, "knowledge_context": false})
}

var rk1PriorLine = regexp.MustCompile(`(?m)^PRIOR_RUN ([0-9a-f-]{36}) RECORD (COMPLETE|INCOMPLETE) RESULT (PRESENT|ABSENT)\s*$`)
var rk1PriorText = regexp.MustCompile(`(?m)^RESULT_TEXT (.*)$`)

// RK1-H1 (narrowed by the runtime addendum): a prior run stopped mid-way with
// a committed checkpoint and no result. The measured run must report it as
// incomplete with no result and must not invent its content.
func rk1H1(f *rk1Fixture, tr *rk1Trial, m rk1Mode) {
	f.agentTarget("agent")
	decoy := "INTERIM-" + rk1Code()
	f.submit(f.owner, "agent", "Prepare an interim analysis of the flaky build and keep working until you are told to stop.")
	f.control(false)
	seed := f.nextRun()
	if f.run(tr, "seed_stopped", false, seed, f.seed(m, "block", map[string]string{"decoy": decoy}), true, f.stopAfterCheckpoint) == nil {
		return
	}
	f.control(false)
	f.submit(f.owner, "agent", "Using only the handoff in this prompt, find the most recent earlier run of this issue. Reply with exactly two lines:\n"+
		"PRIOR_RUN <run_id> RECORD <COMPLETE|INCOMPLETE> RESULT <PRESENT|ABSENT>\nRESULT_TEXT <its recorded result text, or NONE RECORDED if it has none>\n"+
		"Do not guess what that run concluded."+rk1NoWrites)
	o := f.run(tr, "measured", true, f.nextRun(), f.plan(m, "prior", map[string]string{"wrong_answer": "PRIOR_RUN " + seed + " RECORD COMPLETE RESULT PRESENT\nRESULT_TEXT " + decoy}), false, nil)
	if o == nil {
		return
	}
	visible := false
	for _, item := range o.Envelope.Results.Items {
		visible = visible || item.RunID == seed && item.RecordingIncomplete && item.ResultState == "absent"
	}
	if !visible {
		f.incomplete(tr, "precondition:prior_run_not_visible_as_incomplete")
		return
	}
	line, text := rk1PriorLine.FindStringSubmatch(o.answer()), rk1PriorText.FindStringSubmatch(o.answer())
	criteria := map[string]any{"format": line != nil && text != nil, "run_id": line != nil && line[1] == seed,
		"record_incomplete": line != nil && line[2] == "INCOMPLETE", "result_absent": line != nil && line[3] == "ABSENT",
		"no_invented_text": text != nil && strings.TrimSpace(text[1]) == "NONE RECORDED", "decoy_absent": !strings.Contains(o.answer(), decoy),
		// The decoy exists only in checkpoint text no runtime read returns.
		// This stays false unless a new leak path appears; it guards, not scores.
		"decoy_exposed_to_run": o.cliSeen(decoy) || o.toolSaw(decoy)}
	verdict := "pass"
	for _, key := range []string{"format", "run_id", "record_incomplete", "result_absent", "no_invented_text", "decoy_absent"} {
		if criteria[key] != true {
			verdict = "fail"
		}
	}
	f.finish(tr, o, verdict, criteria)
}

var rk1SquadAnswer = regexp.MustCompile(`(?m)^MEMBER_RUN ([0-9a-f-]{36})\s*\nMEMBER_CODE (MEMBER-[A-Z0-9]{6})\s*\nWOKEN_BY (\S+) ([0-9a-f-]{36})\s*$`)

func rk1SquadInput(member string) string {
	return "Coordinate this check through your squad. On your first turn, post exactly one comment that mentions your member as [@Member](mention://agent/" + member +
		") and asks it to check the rollout plan and reply with its verification code, then end the turn. When you are woken again after the member has reported, " +
		"reply with exactly three lines and post nothing:\nMEMBER_RUN <the member's run_id>\nMEMBER_CODE <the member's verification code>\nWOKEN_BY <source_kind> <source_run_id of what woke this turn>\n" +
		"Do not modify files, accounts or collaboration memory and do not start background work."
}

// RK1-H2 / H2b: a measured squad continuation reports the member's result and
// what woke it. H2b puts the decisive code past the 4096 B source prefix, so it
// is reachable only through `multica collaboration handoff source`.
func rk1Squad(tail bool) func(*rk1Fixture, *rk1Trial, rk1Mode) {
	return func(f *rk1Fixture, tr *rk1Trial, m rk1Mode) {
		_, _, member := f.squadTarget("squad")
		code, decoy := "MEMBER-"+rk1Code(), "MEMBER-"+rk1Code()
		report := "Rollout plan checked. Verification code: " + code
		if tail {
			report = "Preliminary verification code " + decoy + " is superseded by the final code at the end of this report.\n" +
				strings.Repeat("Checked canary weights, alert routes and rollback steps; no change from the previous rollout. ", 60) + "\nFinal verification code: " + code
		}
		f.submit(f.owner, "squad", rk1SquadInput(member))
		f.control(false)
		if f.run(tr, "seed_leader", false, f.nextRun(), f.seed(m, "delegate", map[string]string{"comment": "[@Member](mention://agent/" + member + ") Check the rollout plan and reply with your verification code."}), false, nil) == nil {
			return
		}
		memberRun := f.nextRun()
		if f.run(tr, "seed_member", false, memberRun, f.seed(m, "report", map[string]string{"answer": report}), false, nil) == nil {
			return
		}
		behavior := "squad"
		if tail {
			behavior = "squad_tail"
		}
		o := f.run(tr, "measured", true, f.nextRun(), f.plan(m, behavior, map[string]string{"pattern": `(MEMBER-[A-Z0-9]{6})`,
			"wrong_answer": "MEMBER_RUN " + memberRun + "\nMEMBER_CODE " + decoy + "\nWOKEN_BY proof " + memberRun}), false, nil)
		if o == nil {
			return
		}
		visible, truncated := false, false
		for _, source := range o.Envelope.Sources {
			if source.Kind == "provider_result" && source.RunID == memberRun {
				visible, truncated = strings.Contains(source.Content, code), source.Truncated
			}
		}
		if o.Envelope.Assignment.Role != "continuation" || len(o.Envelope.WakeupCauses.Items) == 0 || tail == visible || tail != truncated {
			f.incomplete(tr, "precondition:squad_envelope_shape")
			return
		}
		match := rk1SquadAnswer.FindStringSubmatch(o.answer())
		cause := false
		for _, c := range o.Envelope.WakeupCauses.Items {
			cause = cause || match != nil && c.SourceKind == match[3] && c.SourceRunID == match[4]
		}
		criteria := map[string]any{"format": match != nil, "member_run": match != nil && match[1] == memberRun, "member_code": match != nil && match[2] == code,
			"wake_cause_recorded": cause, "decoy_absent": !strings.Contains(o.answer(), decoy), "wakeup_completeness": o.Envelope.WakeupCauses.Completeness,
			"source_read_through_cli": o.cliSeen(code, "handoff/sources/"+rk1MemberSourceID(o, memberRun)), "full_code_shown_to_model": o.toolSaw(code),
			"member_result_truncated": truncated}
		verdict := "pass"
		for _, key := range []string{"format", "member_run", "member_code", "wake_cause_recorded", "decoy_absent"} {
			if criteria[key] != true {
				verdict = "fail"
			}
		}
		if tail && verdict == "pass" && (criteria["source_read_through_cli"] != true || criteria["full_code_shown_to_model"] != true) {
			verdict = "unsupported_correct_answer"
		}
		f.finish(tr, o, verdict, criteria)
	}
}

func rk1MemberSourceID(o *rk1Obs, run string) string {
	for _, source := range o.Envelope.Sources {
		if source.Kind == "provider_result" && source.RunID == run {
			return source.ID
		}
	}
	return ""
}
