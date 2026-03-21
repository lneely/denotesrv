package disk

import (
	"denotesrv/pkg/encoding/frontmatter"
	"denotesrv/pkg/metadata"
	"denotesrv/pkg/util"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GetFullExtension returns the full extension, including compound extensions.
// For example: "file.md.gpg" returns ".md.gpg", "file.tar.gz" returns ".tar.gz"
func GetFullExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}

	// Check if there's another extension before this one
	pathWithoutExt := strings.TrimSuffix(path, ext)
	innerExt := filepath.Ext(pathWithoutExt)

	if innerExt != "" {
		// Compound extension found, return both parts
		return innerExt + ext
	}

	return ext
}

// GetContentExtension returns the content type extension, stripping encryption/compression layers.
// For example: "file.md.gpg" returns ".md", "file.tar.gz" returns ".tar", "file.md" returns ".md"
func GetContentExtension(path string) string {
	fullExt := GetFullExtension(path)

	// For compound extensions, take the first part
	parts := strings.Split(fullExt, ".")
	if len(parts) >= 3 { // ["", "md", "gpg"] for ".md.gpg"
		return "." + parts[1]
	}

	return fullExt
}

// SupportsFrontMatter returns true if the file extension supports frontmatter.
// Uses the actual extension, not the content extension, so encrypted files (.md.gpg)
// are correctly treated as binary files.
func SupportsFrontMatter(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".org" || ext == ".md" || ext == ".txt"
}

// extractTitleFromContent extracts the title from file content.
// ext should be the file extension (e.g., ".md", ".org", ".txt").
// Returns empty string if no title found or unsupported extension.
func extractTitleFromContent(content string, ext string) string {
	ext = strings.ToLower(ext)
	if ext != ".org" && ext != ".md" && ext != ".txt" {
		return ""
	}

	// Try org-mode #+title: first, then fall back to first heading
	if ext == ".org" {
		if m := regexp.MustCompile(`(?m)^#\+title:\s*(.+)$`).FindStringSubmatch(content); m != nil {
			return strings.TrimSpace(m[1])
		}
		// Fallback to first heading (lines starting with *)
		if m := regexp.MustCompile(`(?m)^\*+\s+(.+)$`).FindStringSubmatch(content); m != nil {
			return strings.TrimSpace(m[1])
		}
	}

	// Try markdown YAML front matter title: first, then fall back to # header
	if ext == ".md" {
		if m := regexp.MustCompile(`(?ms)^---\n.*?^title:\s*(.+?)$.*?^---`).FindStringSubmatch(content); m != nil {
			return strings.TrimSpace(strings.Trim(m[1], `"`))
		}
		if m := regexp.MustCompile(`(?m)^#\s+(.+)$`).FindStringSubmatch(content); m != nil {
			return strings.TrimSpace(m[1])
		}
	}

	return ""
}

// ExtractMetadata extracts metadata from a file (combines filename and content parsing).
// This is the I/O wrapper around metadata's pure functions.
func ExtractMetadata(path string) (*metadata.Metadata, error) {
	// Parse filename (no I/O)
	note := metadata.ParseFilename(path)

	// Check if we should try to read file content for title
	ext := strings.ToLower(filepath.Ext(path))
	// Don't read unsupported file types
	if ext != ".org" && ext != ".md" && ext != ".txt" {
		return note, nil
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		// If we can't read the file, just use filename metadata
		return note, nil
	}

	// Try to extract title from content
	if title := extractTitleFromContent(string(content), ext); title != "" {
		note.Title = title
	}

	return note, nil
}

// ExtractFrontMatter reads a file and parses its front matter.
// Returns the parsed FrontMatter and the detected FileType.
func ExtractFrontMatter(path string) (*metadata.FrontMatter, metadata.FileType, error) {
	ext := strings.ToLower(filepath.Ext(path))

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	return frontmatter.Unmarshal(content, ext)
}

// UpdateFrontMatter updates the front matter in a file.
func UpdateFrontMatter(path string, fm *metadata.FrontMatter, fileType metadata.FileType) error {
	// Only update frontmatter for supported file types
	if !SupportsFrontMatter(path) {
		return nil
	}

	// Read current content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Apply front matter to content
	newContent, err := util.Apply(string(content), fm, fileType)
	if err != nil {
		return fmt.Errorf("failed to apply front matter: %w", err)
	}

	// Write back to file
	return os.WriteFile(path, []byte(newContent), 0644)
}

// LoadAll walks a directory and extracts metadata from all denote files.
func LoadAll(dir string) (metadata.Results, error) {
	var notes metadata.Results

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		note, err := ExtractMetadata(path)
		if err != nil {
			log.Printf("warning: skipping %s: %v", path, err)
			return nil
		}

		// Only include files with valid identifiers
		if note.Identifier != "" {
			// Warn on invalid tags but continue loading
			if invalid := metadata.ValidateTags(note.Tags); len(invalid) > 0 {
				log.Printf("warning: %s:1 invalid tags %v (must be lowercase alphanumeric)", path, invalid)
			}
			notes = append(notes, note)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return notes, nil
}
