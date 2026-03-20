package metadata

// FileType represents supported file formats for denote notes
type FileType string

const (
	FileTypeOrg    FileType = "org"
	FileTypeMdYaml FileType = "md-yaml"
	FileTypeMdToml FileType = "md-toml"
	FileTypeTxt    FileType = "txt"
)

// fileExtensions contains the list of file extensions
// for which Denote should add front matter.
var fileExtensions = map[FileType]string{
	FileTypeOrg:    ".org",
	FileTypeMdYaml: ".md",
	FileTypeMdToml: ".md",
	FileTypeTxt:    ".txt",
}

// GetExtension returns the file extension for a given file type.
func GetExtension(fileType FileType) string {
	return fileExtensions[fileType]
}

// FrontMatter represents parsed front matter from a note
type FrontMatter struct {
	Title      string
	Tags       []string
	Identifier string
	Signature  string
}

// NewFrontMatter creates a new FrontMatter struct from given parameters
func NewFrontMatter(title, signature string, tags []string, identifier string) *FrontMatter {
	return &FrontMatter{
		Title:      title,
		Tags:       tags,
		Identifier: identifier,
		Signature:  signature,
	}
}
