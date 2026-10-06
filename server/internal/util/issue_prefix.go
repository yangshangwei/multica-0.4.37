package util

import (
	"regexp"
	"strings"
)

var legacyIssuePrefixNonAlpha = regexp.MustCompile(`[^a-zA-Z]`)

// ResolveIssuePrefix preserves the stored prefix or the frozen pre-MUL-6050
// name fallback: first three ASCII letters, uppercased, or WS when absent.
// Do not replace this with the slug-derived rule used for new workspaces:
// changing it would rewrite identifiers in existing legacy workspaces.
func ResolveIssuePrefix(storedPrefix, workspaceName string) string {
	if storedPrefix != "" {
		return storedPrefix
	}
	letters := legacyIssuePrefixNonAlpha.ReplaceAllString(workspaceName, "")
	if len(letters) == 0 {
		return "WS"
	}
	letters = strings.ToUpper(letters)
	if len(letters) > 3 {
		letters = letters[:3]
	}
	return letters
}
