package service

import (
	"slices"
	"testing"
)

func TestTemplateLanguagesRetainEnglishAndChinese(t *testing.T) {
	if !slices.Equal(TemplateLanguages, []string{"en", "zh"}) {
		t.Fatalf("active template languages = %v, want [en zh]", TemplateLanguages)
	}
}

func TestTemplateCatalogsRetiredLanguagesFallBackToEnglish(t *testing.T) {
	check := func(name string, title, description func(string) string) {
		t.Helper()
		for _, language := range []string{"ja", "ko"} {
			if title(language) != title("en") || description(language) != description("en") {
				t.Errorf("%s: retired language %q did not fall back to English", name, language)
			}
		}
	}
	for _, template := range AllAgentRoleTemplates() {
		check(template.Key, template.Title, template.Description)
	}
	for _, template := range AutopilotTemplates() {
		check(template.Key, template.Title, template.Description)
		for _, language := range []string{"ja", "ko"} {
			if template.CategoryLabel(language) != template.CategoryLabel("en") {
				t.Errorf("%s: retired category %q did not fall back to English", template.Key, language)
			}
		}
	}
	for _, template := range SquadTemplates() {
		check(template.Key, template.Title, template.Description)
		for _, slot := range template.Members {
			for language := range slot.Roles {
				if language != "en" && language != "zh" {
					t.Errorf("%s/%s: role still contains retired language %q", template.Key, slot.TemplateKey, language)
				}
			}
		}
	}
	for _, template := range McpServerTemplates() {
		check(template.Key, template.Title, template.Description)
	}
}
