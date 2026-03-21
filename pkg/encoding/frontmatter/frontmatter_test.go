package frontmatter

import (
	"denotesrv/pkg/metadata"
	"slices"
	"strings"
	"testing"
)

// TestFormatTags validates tag formatting for different file types
// Maps to dt-denote--format-front-matter from original tests
func TestFormatTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		fileType metadata.FileType
		want     string
	}{
		{
			name:     "org with multiple tags",
			tags:     []string{"tag1", "tag2"},
			fileType: metadata.FileTypeOrg,
			want:     ":tag1:tag2:",
		},
		{
			name:     "org with single tag",
			tags:     []string{"single"},
			fileType: metadata.FileTypeOrg,
			want:     ":single:",
		},
		{
			name:     "org with empty tags",
			tags:     []string{},
			fileType: metadata.FileTypeOrg,
			want:     "",
		},
		{
			name:     "md-yaml with multiple tags",
			tags:     []string{"tag1", "tag2"},
			fileType: metadata.FileTypeMdYaml,
			want:     `["tag1", "tag2"]`,
		},
		{
			name:     "md-yaml with single tag",
			tags:     []string{"single"},
			fileType: metadata.FileTypeMdYaml,
			want:     `["single"]`,
		},
		{
			name:     "md-yaml with empty tags",
			tags:     []string{},
			fileType: metadata.FileTypeMdYaml,
			want:     "",
		},
		{
			name:     "md-toml with multiple tags",
			tags:     []string{"tag1", "tag2"},
			fileType: metadata.FileTypeMdToml,
			want:     `["tag1", "tag2"]`,
		},
		{
			name:     "md-toml with empty tags",
			tags:     []string{},
			fileType: metadata.FileTypeMdToml,
			want:     "",
		},
		{
			name:     "txt with multiple tags",
			tags:     []string{"tag1", "tag2"},
			fileType: metadata.FileTypeTxt,
			want:     "tag1  tag2",
		},
		{
			name:     "txt with single tag",
			tags:     []string{"single"},
			fileType: metadata.FileTypeTxt,
			want:     "single",
		},
		{
			name:     "txt with empty tags",
			tags:     []string{},
			fileType: metadata.FileTypeTxt,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTags(tt.tags, tt.fileType)
			if got != tt.want {
				t.Errorf("formatTags(%v, %q) = %q, want %q", tt.tags, tt.fileType, got, tt.want)
			}
		})
	}
}

// TestFrontMatterBytes validates front matter formatting for all file types
// Maps to dt-denote--format-front-matter from original tests
func TestFrontMatterBytes(t *testing.T) {
	identifier := "20240101T120000"
	title := "Test Note"
	tags := []string{"tag1", "tag2"}

	tests := []struct {
		name            string
		fileType        metadata.FileType
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:     "org format",
			fileType: metadata.FileTypeOrg,
			wantContains: []string{
				"#+title:      Test Note",
				"#+filetags:   :tag1:tag2:",
				"#+identifier: 20240101T120000",
				"#+date:",
			},
		},
		{
			name:     "md-yaml format",
			fileType: metadata.FileTypeMdYaml,
			wantContains: []string{
				"---",
				"title:      Test Note",
				`tags:       ["tag1", "tag2"]`,
				"identifier: 20240101T120000",
				"date:",
			},
		},
		{
			name:     "md-toml format",
			fileType: metadata.FileTypeMdToml,
			wantContains: []string{
				"+++",
				"title      = Test Note",
				`tags       = ["tag1", "tag2"]`,
				"identifier = 20240101T120000",
				"date       =",
			},
		},
		{
			name:     "txt format",
			fileType: metadata.FileTypeTxt,
			wantContains: []string{
				"title:      Test Note",
				"tags:       tag1  tag2",
				"identifier: 20240101T120000",
				"date:",
				"---------------------------",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := metadata.NewFrontMatter(title, "", tags, identifier)
			got := string(Marshal(fm, tt.fileType))

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FrontMatter.Bytes() missing %q\nGot:\n%s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("FrontMatter.Bytes() should not contain %q\nGot:\n%s", notWant, got)
				}
			}
		})
	}
}

// TestFrontMatterBytesEmptyTags validates front matter formatting with no tags
func TestFrontMatterBytesEmptyTags(t *testing.T) {
	identifier := "20240101T120000"
	title := "Test Note"
	tags := []string{}

	tests := []struct {
		name     string
		fileType metadata.FileType
		wantTags string
	}{
		{
			name:     "org with empty tags",
			fileType: metadata.FileTypeOrg,
			wantTags: "#+filetags:",
		},
		{
			name:     "md-yaml with empty tags",
			fileType: metadata.FileTypeMdYaml,
			wantTags: "tags:",
		},
		{
			name:     "md-toml with empty tags",
			fileType: metadata.FileTypeMdToml,
			wantTags: "tags       =",
		},
		{
			name:     "txt with empty tags",
			fileType: metadata.FileTypeTxt,
			wantTags: "tags:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := metadata.NewFrontMatter(title, "", tags, identifier)
			got := string(Marshal(fm, tt.fileType))

			if !strings.Contains(got, tt.wantTags) {
				t.Errorf("FrontMatter.Bytes() with empty tags should contain %q\nGot:\n%s",
					tt.wantTags, got)
			}
		})
	}
}

// TestUnmarshal validates front matter parsing from content
func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		ext            string
		wantTitle      string
		wantTags       []string
		wantIdentifier string
		wantFileType   metadata.FileType
	}{
		{
			name: "org format",
			content: `#+title: Org Note
#+date: [2024-01-01 Mon 12:00]
#+filetags: :work:emacs:
#+identifier: 20240101T120000

* First Heading`,
			ext:            ".org",
			wantTitle:      "Org Note",
			wantTags:       []string{"work", "emacs"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeOrg,
		},
		{
			name: "org with single tag",
			content: `#+title: Single Tag
#+filetags: :single:
#+identifier: 20240101T120000`,
			ext:            ".org",
			wantTitle:      "Single Tag",
			wantTags:       []string{"single"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeOrg,
		},
		{
			name: "org without tags",
			content: `#+title: No Tags
#+identifier: 20240101T120000`,
			ext:            ".org",
			wantTitle:      "No Tags",
			wantTags:       nil,
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeOrg,
		},
		{
			name: "markdown yaml",
			content: `---
title: Markdown Note
date: 2024-01-01
tags: [work, personal]
identifier: 20240101T120000
---

# Content`,
			ext:            ".md",
			wantTitle:      "Markdown Note",
			wantTags:       []string{"work", "personal"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeMdYaml,
		},
		{
			name: "markdown yaml with quoted title",
			content: `---
title: "Quoted Title"
tags: [test]
identifier: 20240101T120000
---`,
			ext:            ".md",
			wantTitle:      "Quoted Title",
			wantTags:       []string{"test"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeMdYaml,
		},
		{
			name: "markdown toml",
			content: `+++
title = TOML Note
date = 2024-01-01
tags = [rust, go]
identifier = 20240101T120000
+++

Content`,
			ext:            ".md",
			wantTitle:      "TOML Note",
			wantTags:       []string{"rust", "go"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeMdToml,
		},
		{
			name: "txt format",
			content: `title: Plain Text
date: 2024-01-01
tags: simple plain
identifier: 20240101T120000
---------------------------

Content here`,
			ext:            ".txt",
			wantTitle:      "Plain Text",
			wantTags:       []string{"simple", "plain"},
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeTxt,
		},
		{
			name: "txt without tags",
			content: `title: No Tags Text
identifier: 20240101T120000
---------------------------`,
			ext:            ".txt",
			wantTitle:      "No Tags Text",
			wantTags:       nil,
			wantIdentifier: "20240101T120000",
			wantFileType:   metadata.FileTypeTxt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, fileType, err := Unmarshal([]byte(tt.content), tt.ext)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Title != tt.wantTitle {
				t.Errorf("Unmarshal().Title = %q, want %q", got.Title, tt.wantTitle)
			}

			if !slices.Equal(got.Tags, tt.wantTags) {
				t.Errorf("Unmarshal().Tags = %v, want %v", got.Tags, tt.wantTags)
			}

			if got.Identifier != tt.wantIdentifier {
				t.Errorf("Unmarshal().Identifier = %q, want %q", got.Identifier, tt.wantIdentifier)
			}

			if fileType != tt.wantFileType {
				t.Errorf("Unmarshal() fileType = %q, want %q", fileType, tt.wantFileType)
			}
		})
	}
}

// TestUnmarshalMissingFields validates handling of missing fields
func TestUnmarshalMissingFields(t *testing.T) {
	content := `---
date: 2024-01-01
---`

	got, _, err := Unmarshal([]byte(content), ".md")
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Should have empty values, not error
	if got.Title != "" {
		t.Errorf("Unmarshal().Title = %q, want empty", got.Title)
	}
	if got.Identifier != "" {
		t.Errorf("Unmarshal().Identifier = %q, want empty", got.Identifier)
	}
	if got.Tags != nil {
		t.Errorf("Unmarshal().Tags = %v, want nil", got.Tags)
	}
}

