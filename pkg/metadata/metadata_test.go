package metadata

import (
	"regexp"
	"slices"
	"testing"
	"time"
)

// TestGenerateIdentifier validates timestamp format (YYYYMMDDThhmmss)
// Maps to dt-denote-identifier-p from original tests
func TestGenerateIdentifier(t *testing.T) {
	id := GenerateIdentifier()

	// Validate format
	pattern := regexp.MustCompile(`^\d{8}T\d{6}$`)
	if !pattern.MatchString(id) {
		t.Errorf("GenerateIdentifier() = %q, want format YYYYMMDDThhmmss", id)
	}

	// Validate it's a valid timestamp
	_, err := time.Parse("20060102T150405", id)
	if err != nil {
		t.Errorf("GenerateIdentifier() = %q, not a valid timestamp: %v", id, err)
	}
}

// TestSlugifyTitle validates title slugification
// Maps to dt-denote-sluggify-title and dt-denote-sluggify from original tests
func TestSlugifyTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic spaces to hyphens",
			input: "Hello World",
			want:  "hello-world",
		},
		{
			name:  "underscores to hyphens",
			input: "Test_File Name",
			want:  "test-file-name",
		},
		{
			name:  "remove punctuation",
			input: "Test File!",
			want:  "test-file",
		},
		{
			name:  "mixed case",
			input: "Mixed CASE 123",
			want:  "mixed-case-123",
		},
		{
			name:  "special characters",
			input: "Special@#$Chars",
			want:  "specialchars",
		},
		{
			name:  "multiple spaces",
			input: "Multiple   Spaces",
			want:  "multiple---spaces",
		},
		{
			name:  "leading and trailing spaces",
			input: "  Trim Me  ",
			want:  "--trim-me--",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "numbers only",
			input: "12345",
			want:  "12345",
		},
		{
			name:  "hyphens preserved",
			input: "already-hyphenated",
			want:  "already-hyphenated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugifyTitle(tt.input)
			if got != tt.want {
				t.Errorf("slugifyTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFormatKeywords validates keyword formatting for filenames
// Maps to dt-denote-sluggify-keywords from original tests
func TestFormatKeywords(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "multiple keywords",
			input: []string{"tag1", "tag2"},
			want:  "__tag1_tag2",
		},
		{
			name:  "single keyword",
			input: []string{"single"},
			want:  "__single",
		},
		{
			name:  "empty keywords",
			input: []string{},
			want:  "",
		},
		{
			name:  "nil keywords",
			input: nil,
			want:  "",
		},
		{
			name:  "three keywords",
			input: []string{"one", "two", "three"},
			want:  "__one_two_three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKeywords(tt.input)
			if got != tt.want {
				t.Errorf("formatKeywords(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSlugifySignature validates signature slugification
func TestSlugifySignature(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic spaces to equals",
			input: "Hello World",
			want:  "hello==world",
		},
		{
			name:  "underscores to equals",
			input: "test_file",
			want:  "test==file",
		},
		{
			name:  "remove special characters",
			input: "a!@#b",
			want:  "ab",
		},
		{
			name:  "normalize consecutive equals",
			input: "a===b",
			want:  "a==b",
		},
		{
			name:  "mixed",
			input: "Hello World_Test",
			want:  "hello==world==test",
		},
		{
			name:  "trim trailing equals",
			input: "test_",
			want:  "test",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "lowercase already",
			input: "already",
			want:  "already",
		},
		{
			name:  "numbers preserved",
			input: "test123",
			want:  "test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugifySignature(tt.input)
			if got != tt.want {
				t.Errorf("slugifySignature(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestBuildFilename validates filename construction
// Maps to dt-denote-format-file-name from original tests
func TestBuildFilename(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		signature  string
		title      string
		keywords   []string
		ext        string
		want       string
		ftype      FileType
	}{
		{
			name:       "complete filename with keywords",
			identifier: "20231225T120000",
			signature:  "",
			title:      "My Title",
			keywords:   []string{"tag1", "tag2"},
			ext:        ".md",
			want:       "20231225T120000--my-title__tag1_tag2.md",
			ftype:      FileTypeMdYaml,
		},
		{
			name:       "filename without keywords",
			identifier: "20231225T120000",
			signature:  "",
			title:      "My Title",
			keywords:   []string{},
			ext:        ".md",
			want:       "20231225T120000--my-title.md",
			ftype:      FileTypeMdYaml,
		},
		{
			name:       "filename with signature",
			identifier: "20231225T120000",
			signature:  "hello",
			title:      "My Title",
			keywords:   []string{"tag1"},
			ext:        ".md",
			want:       "20231225T120000==hello--my-title__tag1.md",
			ftype:      FileTypeMdYaml,
		},
		{
			name:       "filename with signature and no keywords",
			identifier: "20231225T120000",
			signature:  "test",
			title:      "My Title",
			keywords:   []string{},
			ext:        ".md",
			want:       "20231225T120000==test--my-title.md",
			ftype:      FileTypeMdYaml,
		},
		{
			name:       "filename with multi-part signature",
			identifier: "20231225T120000",
			signature:  "a==b",
			title:      "My Title",
			keywords:   []string{"work"},
			ext:        ".md",
			want:       "20231225T120000==a==b--my-title__work.md",
			ftype:      FileTypeMdYaml,
		},
		{
			name:       "filename with special chars in title",
			identifier: "20231225T120000",
			signature:  "",
			title:      "Special!@# Title",
			keywords:   []string{"work"},
			ext:        ".org",
			want:       "20231225T120000--special-title__work.org",
			ftype:      FileTypeOrg,
		},
		{
			name:       "org format",
			identifier: "20240101T000000",
			signature:  "",
			title:      "Org Note",
			keywords:   []string{"emacs"},
			ext:        ".org",
			want:       "20240101T000000--org-note__emacs.org",
			ftype:      FileTypeOrg,
		},
		{
			name:       "txt format",
			identifier: "20240101T000000",
			signature:  "",
			title:      "Plain Text",
			keywords:   []string{},
			ext:        ".txt",
			want:       "20240101T000000--plain-text.txt",
			ftype:      FileTypeTxt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFrontMatter(tt.title, tt.signature, tt.keywords, tt.identifier)
			got := BuildFilename(fm, tt.ext)
			if got != tt.want {
				t.Errorf("BuildFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseFilename validates filename parsing
// Maps to dt-denote-retrieve-filename-* tests from original
func TestParseFilename(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		wantIdentifier string
		wantSignature  string
		wantTitle      string
		wantTags       []string
	}{
		{
			name:           "complete filename with tags",
			path:           "/home/notes/20231225T120000--my-title__tag1_tag2.md",
			wantIdentifier: "20231225T120000",
			wantSignature:  "",
			wantTitle:      "my title",
			wantTags:       []string{"tag1", "tag2"},
		},
		{
			name:           "filename without tags",
			path:           "/home/notes/20231225T120000--simple-title.md",
			wantIdentifier: "20231225T120000",
			wantSignature:  "",
			wantTitle:      "simple title",
			wantTags:       nil,
		},
		{
			name:           "filename with signature",
			path:           "20240101T000000==hello--note__work.org",
			wantIdentifier: "20240101T000000",
			wantSignature:  "hello",
			wantTitle:      "note",
			wantTags:       []string{"work"},
		},
		{
			name:           "filename with signature and no tags",
			path:           "20240101T000000==test--title.md",
			wantIdentifier: "20240101T000000",
			wantSignature:  "test",
			wantTitle:      "title",
			wantTags:       nil,
		},
		{
			name:           "filename with multi-part signature",
			path:           "20240101T000000==a==b--note__tag.md",
			wantIdentifier: "20240101T000000",
			wantSignature:  "a==b",
			wantTitle:      "note",
			wantTags:       []string{"tag"},
		},
		{
			name:           "filename with single tag",
			path:           "20240101T000000--note__work.org",
			wantIdentifier: "20240101T000000",
			wantSignature:  "",
			wantTitle:      "note",
			wantTags:       []string{"work"},
		},
		{
			name:           "identifier only",
			path:           "20240101T000000.txt",
			wantIdentifier: "20240101T000000",
			wantSignature:  "",
			wantTitle:      "",
			wantTags:       nil,
		},
		{
			name:           "multi-word title",
			path:           "20240101T000000--multi-word-title__personal_ideas.md",
			wantIdentifier: "20240101T000000",
			wantSignature:  "",
			wantTitle:      "multi word title",
			wantTags:       []string{"personal", "ideas"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFilename(tt.path)

			if got.Identifier != tt.wantIdentifier {
				t.Errorf("ParseFilename(%q).Identifier = %q, want %q",
					tt.path, got.Identifier, tt.wantIdentifier)
			}

			if got.Signature != tt.wantSignature {
				t.Errorf("ParseFilename(%q).Signature = %q, want %q",
					tt.path, got.Signature, tt.wantSignature)
			}

			if got.Title != tt.wantTitle {
				t.Errorf("ParseFilename(%q).Title = %q, want %q",
					tt.path, got.Title, tt.wantTitle)
			}

			if !slices.Equal(got.Tags, tt.wantTags) {
				t.Errorf("ParseFilename(%q).Tags = %v, want %v",
					tt.path, got.Tags, tt.wantTags)
			}

			if got.Path != tt.path {
				t.Errorf("ParseFilename(%q).Path = %q, want %q",
					tt.path, got.Path, tt.path)
			}
		})
	}
}

// TestSort validates sorting functionality
func TestSort(t *testing.T) {
	notes := Results{
		{Identifier: "20240103T120000", Title: "Charlie"},
		{Identifier: "20240101T120000", Title: "Alice"},
		{Identifier: "20240102T120000", Title: "Bob"},
	}

	t.Run("sort by id ascending", func(t *testing.T) {
		testData := make(Results, len(notes))
		copy(testData, notes)

		Sort(testData, SortById, SortOrderAsc)

		if testData[0].Identifier != "20240101T120000" {
			t.Errorf("First item identifier = %q, want %q", testData[0].Identifier, "20240101T120000")
		}
		if testData[2].Identifier != "20240103T120000" {
			t.Errorf("Last item identifier = %q, want %q", testData[2].Identifier, "20240103T120000")
		}
	})

	t.Run("sort by id descending", func(t *testing.T) {
		testData := make(Results, len(notes))
		copy(testData, notes)

		Sort(testData, SortById, SortOrderDesc)

		if testData[0].Identifier != "20240103T120000" {
			t.Errorf("First item identifier = %q, want %q", testData[0].Identifier, "20240103T120000")
		}
		if testData[2].Identifier != "20240101T120000" {
			t.Errorf("Last item identifier = %q, want %q", testData[2].Identifier, "20240101T120000")
		}
	})

	t.Run("sort by title ascending", func(t *testing.T) {
		testData := make(Results, len(notes))
		copy(testData, notes)

		Sort(testData, SortByTitle, SortOrderAsc)

		if testData[0].Title != "Alice" {
			t.Errorf("First item title = %q, want %q", testData[0].Title, "Alice")
		}
		if testData[2].Title != "Charlie" {
			t.Errorf("Last item title = %q, want %q", testData[2].Title, "Charlie")
		}
	})

	t.Run("sort by title descending", func(t *testing.T) {
		testData := make(Results, len(notes))
		copy(testData, notes)

		Sort(testData, SortByTitle, SortOrderDesc)

		if testData[0].Title != "Charlie" {
			t.Errorf("First item title = %q, want %q", testData[0].Title, "Charlie")
		}
		if testData[2].Title != "Alice" {
			t.Errorf("Last item title = %q, want %q", testData[2].Title, "Alice")
		}
	})

	t.Run("sort by title case insensitive", func(t *testing.T) {
		testData := Results{
			{Identifier: "1", Title: "zebra"},
			{Identifier: "2", Title: "Apple"},
			{Identifier: "3", Title: "banana"},
		}

		Sort(testData, SortByTitle, SortOrderAsc)

		// Should be: Apple, banana, zebra (case-insensitive)
		if testData[0].Title != "Apple" {
			t.Errorf("First item title = %q, want %q", testData[0].Title, "Apple")
		}
		if testData[1].Title != "banana" {
			t.Errorf("Second item title = %q, want %q", testData[1].Title, "banana")
		}
		if testData[2].Title != "zebra" {
			t.Errorf("Third item title = %q, want %q", testData[2].Title, "zebra")
		}
	})
}