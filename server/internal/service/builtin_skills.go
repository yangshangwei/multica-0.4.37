package service

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

//go:embed builtin_skills
var builtinSkillsFS embed.FS

const builtinSkillsRoot = "builtin_skills"
const mikaOnboardingSkillName = "multica-onboarding"

// BuiltinSkills returns the platform's built-in skills, embedded at compile
// time, excluding task-specific onboarding. Every agent receives these on top
// of its workspace-bound skills. They teach "how to" workflows (e.g. mentioning)
// that the runtime brief intentionally leaves to skills.
//
// Layout: builtin_skills/<name>/SKILL.md plus optional supporting files. The
// <name> directory carries a "multica-" prefix so its on-disk slug can never
// collide with a workspace skill a user authored (see writeSkillFiles, which
// derives the skill directory from AgentSkillData.Name).
func (s *TaskService) BuiltinSkills() []AgentSkillData {
	return slices.DeleteFunc(loadBuiltinSkills(), func(skill AgentSkillData) bool {
		return skill.Name == mikaOnboardingSkillName
	})
}

// TaskBuiltinSkills includes onboarding only in Mika conversations with a
// product-authored kickoff. Session provenance keeps follow-up and retry turns
// eligible without trusting display names, message text, or input ownership.
func (s *TaskService) TaskBuiltinSkills(ctx context.Context, task db.AgentTaskQueue) ([]AgentSkillData, error) {
	skills := s.BuiltinSkills()
	if !task.ChatSessionID.Valid {
		return skills, nil
	}
	onboarding, err := s.Queries.ChatSessionHasOnboardingKickoff(ctx, db.ChatSessionHasOnboardingKickoffParams{
		ChatSessionID: task.ChatSessionID,
		AgentID:       task.AgentID,
		SystemKey:     pgtype.Text{String: MikaSystemKey, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("check onboarding skill scope: %w", err)
	}
	if onboarding {
		skill, ok := loadBuiltinSkill(mikaOnboardingSkillName)
		if !ok {
			return nil, fmt.Errorf("onboarding skill is not embedded")
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func loadBuiltinSkills() []AgentSkillData {
	entries, err := fs.ReadDir(builtinSkillsFS, builtinSkillsRoot)
	if err != nil {
		return nil
	}
	var skills []AgentSkillData
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if skill, ok := loadBuiltinSkill(entry.Name()); ok {
			skills = append(skills, skill)
		}
	}
	return skills
}

func loadBuiltinSkill(name string) (AgentSkillData, bool) {
	dir := path.Join(builtinSkillsRoot, name)
	content, err := fs.ReadFile(builtinSkillsFS, path.Join(dir, "SKILL.md"))
	if err != nil {
		// A skill directory without a SKILL.md is malformed — skip it rather
		// than ship an empty skill.
		return AgentSkillData{}, false
	}
	skill := AgentSkillData{Name: name, Content: string(content)}
	// Any other file in the directory becomes a supporting file, preserving
	// its relative path so subdirectories (e.g. rules/styling.md) survive.
	_ = fs.WalkDir(builtinSkillsFS, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == "SKILL.md" {
			return nil
		}
		data, readErr := fs.ReadFile(builtinSkillsFS, p)
		if readErr != nil {
			return nil
		}
		skill.Files = append(skill.Files, AgentSkillFileData{Path: rel, Content: string(data)})
		return nil
	})
	return skill, true
}
