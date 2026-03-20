package util

import (
	"denotesrv/pkg/encoding/frontmatter"
	"denotesrv/pkg/metadata"
	"fmt"
	"regexp"
	"strings"
)

// Apply applies front matter to file content, replacing existing front matter if present.
// originalContent is the current file content, fm is the new front matter to apply.
func Apply(originalContent string, fm *metadata.FrontMatter, fileType metadata.FileType) (string, error) {
	text := originalContent
	newFrontMatter := string(frontmatter.Marshal(fm, fileType))

	var newText string
	switch fileType {
	case metadata.FileTypeOrg:
		// Find end of front matter (first blank line or non-#+ line)
		lines := strings.Split(text, "\n")
		endIdx := 0
		for i, line := range lines {
			if i > 0 && (line == "" || !strings.HasPrefix(line, "#+")) {
				endIdx = i
				break
			}
		}
		// Skip any blank lines after frontmatter since template includes one
		for endIdx < len(lines) && lines[endIdx] == "" {
			endIdx++
		}
		if endIdx > 0 {
			newText = newFrontMatter + strings.Join(lines[endIdx:], "\n")
		} else {
			newText = newFrontMatter + text
		}

	case metadata.FileTypeMdYaml:
		// Replace YAML front matter (match trailing blank lines to avoid duplication)
		re := regexp.MustCompile(`(?s)^---\n.*?\n---\n\n*`)
		if re.MatchString(text) {
			newText = re.ReplaceAllString(text, newFrontMatter)
		} else {
			newText = newFrontMatter + text
		}

	case metadata.FileTypeMdToml:
		// Replace TOML front matter (match trailing blank lines to avoid duplication)
		re := regexp.MustCompile(`(?s)^\+\+\+\n.*?\n\+\+\+\n\n*`)
		if re.MatchString(text) {
			newText = re.ReplaceAllString(text, newFrontMatter)
		} else {
			newText = newFrontMatter + text
		}

	case metadata.FileTypeTxt:
		// Replace text front matter (match trailing blank lines to avoid duplication)
		re := regexp.MustCompile(`(?s)^title:.*?\n-+\n\n*`)
		if re.MatchString(text) {
			newText = re.ReplaceAllString(text, newFrontMatter)
		} else {
			newText = newFrontMatter + text
		}
	default:
		return "", fmt.Errorf("unsupported file type: %s", fileType)
	}

	return newText, nil
}
