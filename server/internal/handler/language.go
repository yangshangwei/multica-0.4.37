package handler

// normalizeRetiredLanguage keeps installed clients and stored preferences
// compatible without advertising retired translations or accepting new aliases.
// Each caller keeps its own validation and whitespace rules.
func normalizeRetiredLanguage(language string) string {
	switch language {
	case "ja", "ko":
		return "en"
	default:
		return language
	}
}
