package results

import (
	"denotesrv/pkg/metadata"
	"slices"
	"testing"
)

// TestMarshal validates serialization
func TestMarshal(t *testing.T) {
	tests := []struct {
		name  string
		input metadata.Results
		want  string
	}{
		{
			name: "single note with tags",
			input: metadata.Results{
				{
					Identifier: "20240101T120000",
					Title:      "Test Note",
					Tags:       []string{"tag1", "tag2"},
				},
			},
			want: "20240101T120000 | Test Note | tag1,tag2\n",
		},
		{
			name: "note without tags",
			input: metadata.Results{
				{
					Identifier: "20240101T120000",
					Title:      "Simple Note",
					Tags:       []string{},
				},
			},
			want: "20240101T120000 | Simple Note | \n",
		},
		{
			name: "note without title",
			input: metadata.Results{
				{
					Identifier: "20240101T120000",
					Title:      "",
					Tags:       []string{"tag"},
				},
			},
			want: "20240101T120000 | (untitled) | tag\n",
		},
		{
			name: "multiple notes",
			input: metadata.Results{
				{Identifier: "20240101T120000", Title: "First", Tags: []string{"a"}},
				{Identifier: "20240102T120000", Title: "Second", Tags: []string{"b", "c"}},
			},
			want: "20240101T120000 | First | a\n20240102T120000 | Second | b,c\n",
		},
		{
			name:  "empty results",
			input: metadata.Results{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Marshal(tt.input))
			if got != tt.want {
				t.Errorf("Marshal() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUnmarshal validates parsing from byte format
func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    metadata.Results
		wantErr bool
	}{
		{
			name:  "single note with tags",
			input: []byte("20240101T120000 | Test Note | tag1,tag2"),
			want: metadata.Results{
				{Identifier: "20240101T120000", Title: "Test Note", Tags: []string{"tag1", "tag2"}},
			},
			wantErr: false,
		},
		{
			name:  "note without tags",
			input: []byte("20240101T120000 | Simple Note | "),
			want: metadata.Results{
				{Identifier: "20240101T120000", Title: "Simple Note", Tags: []string{}},
			},
			wantErr: false,
		},
		{
			name:  "multiple notes",
			input: []byte("20240101T120000 | First | a\n20240102T120000 | Second | b,c"),
			want: metadata.Results{
				{Identifier: "20240101T120000", Title: "First", Tags: []string{"a"}},
				{Identifier: "20240102T120000", Title: "Second", Tags: []string{"b", "c"}},
			},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   []byte(""),
			want:    nil,
			wantErr: false,
		},
		{
			name:    "wrong column count",
			input:   []byte("20240101T120000 | Title"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty identifier",
			input:   []byte(" | Title | tags"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid tags with spaces",
			input:   []byte("20240101T120000 | Title | tag with spaces"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid tags with uppercase",
			input:   []byte("20240101T120000 | Title | Tag1,tag2"),
			want:    nil,
			wantErr: true,
		},
		{
			name:  "valid lowercase unicode tags",
			input: []byte("20240101T120000 | Title | tag1,测试,αβγ"),
			want: metadata.Results{
				{Identifier: "20240101T120000", Title: "Title", Tags: []string{"tag1", "测试", "αβγ"}},
			},
			wantErr: false,
		},
		{
			name:  "blank lines ignored",
			input: []byte("20240101T120000 | First | a\n\n20240102T120000 | Second | b"),
			want: metadata.Results{
				{Identifier: "20240101T120000", Title: "First", Tags: []string{"a"}},
				{Identifier: "20240102T120000", Title: "Second", Tags: []string{"b"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalStrict(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("Unmarshal() length = %d, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].Identifier != tt.want[i].Identifier {
					t.Errorf("Result[%d].Identifier = %q, want %q", i, got[i].Identifier, tt.want[i].Identifier)
				}
				if got[i].Title != tt.want[i].Title {
					t.Errorf("Result[%d].Title = %q, want %q", i, got[i].Title, tt.want[i].Title)
				}
				if !slices.Equal(got[i].Tags, tt.want[i].Tags) {
					t.Errorf("Result[%d].Tags = %v, want %v", i, got[i].Tags, tt.want[i].Tags)
				}
			}
		})
	}
}
