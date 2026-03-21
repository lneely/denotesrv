package util

import (
	"denotesrv/pkg/metadata"
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	tests := []struct {
		name         string
		original     string
		fm           *metadata.FrontMatter
		fileType     metadata.FileType
		wantContains []string
		wantPreserve string
	}{
		{
			name: "update org front matter",
			original: `#+title: Old Title
#+date: [2024-01-01 Mon 12:00]
#+filetags: :old:
#+identifier: 20240101T120000

* Original Content
This should be preserved`,
			fm: &metadata.FrontMatter{
				Title:      "New Title",
				Tags:       []string{"new", "updated"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeOrg,
			wantContains: []string{
				"#+title:      New Title",
				"#+filetags:   :new:updated:",
				"* Original Content",
				"This should be preserved",
			},
			wantPreserve: "* Original Content",
		},
		{
			name: "update markdown yaml front matter",
			original: `---
title: Old Title
tags: [old]
identifier: 20240101T120000
---

# Original Heading
Content preserved`,
			fm: &metadata.FrontMatter{
				Title:      "New Title",
				Tags:       []string{"new"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeMdYaml,
			wantContains: []string{
				"---",
				"title:      New Title",
				`tags:       ["new"]`,
				"# Original Heading",
				"Content preserved",
			},
			wantPreserve: "# Original Heading",
		},
		{
			name: "update markdown toml front matter",
			original: `+++
title = Old Title
tags = [old]
identifier = 20240101T120000
+++

Content here`,
			fm: &metadata.FrontMatter{
				Title:      "New Title",
				Tags:       []string{"updated"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeMdToml,
			wantContains: []string{
				"+++",
				"title      = New Title",
				`tags       = ["updated"]`,
				"Content here",
			},
			wantPreserve: "Content here",
		},
		{
			name: "update txt front matter",
			original: `title: Old Title
tags: old
identifier: 20240101T120000
---------------------------

Text content`,
			fm: &metadata.FrontMatter{
				Title:      "New Title",
				Tags:       []string{"new"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeTxt,
			wantContains: []string{
				"title:      New Title",
				"tags:       new",
				"---------------------------",
				"Text content",
			},
			wantPreserve: "Text content",
		},
		{
			name:     "add front matter when missing (org)",
			original: `* Original Heading`,
			fm: &metadata.FrontMatter{
				Title:      "Added Title",
				Tags:       []string{"added"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeOrg,
			wantContains: []string{
				"#+title:      Added Title",
				"#+filetags:   :added:",
				"* Original Heading",
			},
			wantPreserve: "* Original Heading",
		},
		{
			name:     "add front matter when missing (md-yaml)",
			original: `# Original Heading`,
			fm: &metadata.FrontMatter{
				Title:      "Added Title",
				Tags:       []string{"added"},
				Identifier: "20240101T120000",
			},
			fileType: metadata.FileTypeMdYaml,
			wantContains: []string{
				"---",
				"title:      Added Title",
				`tags:       ["added"]`,
				"# Original Heading",
			},
			wantPreserve: "# Original Heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.original, tt.fm, tt.fileType)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Apply() missing %q\nGot:\n%s", want, got)
				}
			}

			// Verify original content is preserved
			if !strings.Contains(got, tt.wantPreserve) {
				t.Errorf("Apply() didn't preserve %q\nGot:\n%s", tt.wantPreserve, got)
			}
		})
	}
}

// TestApplyEmptyTags validates updating with empty tags
func TestApplyEmptyTags(t *testing.T) {
	original := `---
title: Test
tags: [old, tags]
identifier: 20240101T120000
---

Content`

	fm := &metadata.FrontMatter{
		Title:      "Test",
		Tags:       []string{},
		Identifier: "20240101T120000",
	}

	got, err := Apply(original, fm, metadata.FileTypeMdYaml)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Should have empty tags field, not omit it
	if !strings.Contains(got, "tags:") {
		t.Errorf("Apply() should include tags field even when empty")
	}

	// Content should be preserved
	if !strings.Contains(got, "Content") {
		t.Errorf("Apply() should preserve content")
	}
}

// TestApplyUnsupportedType validates error handling
func TestApplyUnsupportedType(t *testing.T) {
	fm := &metadata.FrontMatter{
		Title:      "Test",
		Tags:       []string{"test"},
		Identifier: "20240101T120000",
	}

	_, err := Apply("content", fm, "unsupported")
	if err == nil {
		t.Error("Apply() should error on unsupported file type")
	}

	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("Apply() error = %v, want 'unsupported file type'", err)
	}
}
