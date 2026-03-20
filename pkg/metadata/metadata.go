package metadata

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Metadata is the metadata encoded into Denote-style
// file names.
type Metadata struct {
	Path       string
	Identifier string
	Signature  string
	Title      string
	Tags       []string
}

type Results []*Metadata

type SortBy string

const (
	SortById    SortBy = "id"
	SortByDate  SortBy = "date"
	SortByTitle SortBy = "title"
)

type SortOrder int

const (
	SortOrderAsc SortOrder = iota
	SortOrderDesc
)

// Sort organizes a list of notes by sortType and order using metadata.
func Sort(md Results, sortType SortBy, order SortOrder) {
	switch sortType {
	case SortById, SortByDate:
		sort.Slice(md, func(i, j int) bool {
			if order == SortOrderAsc {
				return md[i].Identifier < md[j].Identifier // Reverse chronological by default
			} else {
				return md[i].Identifier > md[j].Identifier // Reverse chronological by default
			}
		})
	case SortByTitle:
		sort.Slice(md, func(i, j int) bool {
			if order == SortOrderAsc {
				return strings.ToLower(md[i].Title) < strings.ToLower(md[j].Title)
			} else {
				return strings.ToLower(md[i].Title) > strings.ToLower(md[j].Title)
			}
		})
	default:
		sort.Slice(md, func(i, j int) bool {
			return md[i].Identifier > md[j].Identifier // Reverse chronological by default
		})
	}
}

// ParseFilename extracts Denote metadata from a filename only (no file I/O).
// Returns metadata with Path, Identifier, Signature, Title (from filename), and Tags.
func ParseFilename(path string) *Metadata {
	fname := filepath.Base(path)
	note := &Metadata{Path: path}

	if m := regexp.MustCompile(`^(\d{8}T\d{6})`).FindStringSubmatch(fname); m != nil {
		note.Identifier = m[1]
	}

	// Extract signature (optional component between identifier and title)
	if m := regexp.MustCompile(`==([^-]+?)--`).FindStringSubmatch(fname); m != nil {
		note.Signature = m[1]
	}

	// Extract title from filename
	if m := regexp.MustCompile(`--([^_\.]+)`).FindStringSubmatch(fname); m != nil {
		note.Title = strings.ReplaceAll(m[1], "-", " ")
	}

	if m := regexp.MustCompile(`__(.+?)(?:\.|$)`).FindStringSubmatch(fname); m != nil {
		note.Tags = strings.Split(m[1], "_")
	}

	return note
}

// IsValidTag returns true if the tag contains only lowercase letters, other unicode letters, or digits.
func IsValidTag(tag string) bool {
	for _, r := range tag {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
		if unicode.IsLetter(r) && unicode.IsUpper(r) {
			return false
		}
	}
	return len(tag) > 0
}

// ValidateTags checks all tags and returns invalid ones.
func ValidateTags(tags []string) []string {
	var invalid []string
	for _, tag := range tags {
		if !IsValidTag(tag) {
			invalid = append(invalid, tag)
		}
	}
	return invalid
}

// GenerateIdentifier creates a new identifier timestamp.
func GenerateIdentifier() string {
	return time.Now().Format("20060102T150405")
}

// slugifyTitle converts a title to a filesystem-safe slug.
func slugifyTitle(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	return regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
}

// slugifySignature converts a signature to Denote-compliant format.
// Following Denote rules: lowercase, spaces/underscores -> double equals,
// remove special chars, normalize consecutive equals to double equals.
func slugifySignature(sig string) string {
	slug := strings.ToLower(sig)
	slug = strings.ReplaceAll(slug, " ", "==")
	slug = strings.ReplaceAll(slug, "_", "==")
	// Remove special characters per Denote spec
	slug = regexp.MustCompile(`[{}!@#$%^&*()+'"?,.\\|;:~\x60''""/-]`).ReplaceAllString(slug, "")
	// Normalize consecutive equals signs (3 or more) to double equals
	slug = regexp.MustCompile(`={3,}`).ReplaceAllString(slug, "==")
	// Trim trailing equals
	slug = strings.Trim(slug, "=")
	return slug
}

// formatKeywords formats keywords for a denote filename.
func formatKeywords(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	return "__" + strings.Join(keywords, "_")
}

// formatSignature formats a signature for a denote filename.
// Returns empty string if signature is empty, otherwise returns ==signature.
func formatSignature(sig string) string {
	if sig == "" {
		return ""
	}
	return "==" + slugifySignature(sig)
}

// BuildFilename constructs a denote filename from metadata components.
func BuildFilename(fm *FrontMatter, ext string) string {
	titleSlug := slugifyTitle(fm.Title)
	signaturePart := formatSignature(fm.Signature)
	keywordsPart := formatKeywords(fm.Tags)
	return fmt.Sprintf("%s%s--%s%s%s", fm.Identifier, signaturePart, titleSlug, keywordsPart, ext)
}
