package results

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"

	"denotesrv/pkg/metadata"
)

// Marshal serializes Results to a pipe-delimited byte format.
// Format: identifier | title | tags (comma-separated)
func Marshal(rs metadata.Results) []byte {
	var buf strings.Builder
	for _, e := range rs {
		title := e.Title
		if title == "" {
			title = "(untitled)"
		}

		tags := strings.Join(e.Tags, ",")
		fmt.Fprintf(&buf, "%s | %s | %s\n", e.Identifier, title, tags)
	}
	return []byte(buf.String())
}

// Unmarshal parses pipe-delimited byte data into Results.
// Format: identifier | title | tags (comma-separated)
// Invalid tags produce warnings but parsing continues.
func Unmarshal(data []byte) (metadata.Results, error) {
	return unmarshal(data, false)
}

// UnmarshalStrict parses pipe-delimited byte data into Results with strict tag validation.
// Returns an error if any tags are invalid.
func UnmarshalStrict(data []byte) (metadata.Results, error) {
	return unmarshal(data, true)
}

func unmarshal(data []byte, strict bool) (metadata.Results, error) {
	var results metadata.Results
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	// Allow lowercase Latin letters, other letters (CJK, etc.), and digits, no spaces
	tagPattern := regexp.MustCompile(`^([\p{Ll}\p{Lo}\p{Nd}]+,)*[\p{Ll}\p{Lo}\p{Nd}]+$`)

	for lineNum, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte("|"))
		if len(parts) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 columns, got %d (line: %q)", lineNum+1, len(parts), line)
		}

		identifier := string(bytes.TrimSpace(parts[0]))
		title := string(bytes.TrimSpace(parts[1]))
		tagsStr := string(bytes.TrimSpace(parts[2]))

		if identifier == "" {
			return nil, fmt.Errorf("line %d: identifier cannot be empty", lineNum+1)
		}

		var tags []string
		if tagsStr != "" {
			if !tagPattern.MatchString(tagsStr) {
				if strict {
					return nil, fmt.Errorf("line %d: tags must be comma-delimited lowercase unicode words (no spaces): got '%s'", lineNum+1, tagsStr)
				}
				log.Printf("warning: line %d: invalid tags '%s' (must be lowercase alphanumeric)", lineNum+1, tagsStr)
			}
			tags = strings.Split(tagsStr, ",")
		} else {
			tags = []string{}
		}

		results = append(results, &metadata.Metadata{
			Identifier: identifier,
			Title:      title,
			Tags:       tags,
		})
	}

	return results, nil
}
