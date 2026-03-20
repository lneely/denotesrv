package metadata

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Filter matches a given field in a Result to a regular expression
type Filter struct {
	field  FilterField
	re     *regexp.Regexp
	negate bool
}

type Filters []*Filter

type FilterField string

const (
	FilterDate  FilterField = "date"
	FilterTitle FilterField = "title"
	FilterTag   FilterField = "tag"
	FilterAny   FilterField = ""
)

// Parse converts a slice of strings of the form "tag:<tagname>",
// "date:<date>", "title:'<title>'" into a Filters list
func (fs Filters) Parse(S []string) (Filters, error) {
	for _, fa := range S {
		f, err := NewFilter(fa)
		if err != nil {
			return nil, fmt.Errorf("failed to list notes: %w", err)
		}
		fs = append(fs, f)
	}
	return fs, nil
}

// NewFilter constructs a Filter from a filter string. arg takes the form
// field:criteria, e.g., tag:/dev|meeting/, date:20251101.
func NewFilter(arg string) (*Filter, error) {
	negate := strings.HasPrefix(arg, "!")
	if negate {
		arg = strings.TrimPrefix(arg, "!")
	}

	m := regexp.MustCompile(`^(?:(date|title|tag):)?(.+)$`).FindStringSubmatch(arg)
	if m == nil {
		return nil, fmt.Errorf("invalid filter syntax: %s", arg)
	}

	fieldStr := m[1]
	value := m[2]

	// Strip surrounding quotes (both single and double)
	value = strings.Trim(value, `"'`)

	// Validate: if field is title and value has spaces, original should have been quoted
	if fieldStr == "title" && strings.Contains(value, " ") {
		// Check if original value was quoted
		if !strings.HasPrefix(m[2], `"`) && !strings.HasPrefix(m[2], `'`) {
			return nil, fmt.Errorf("title with spaces must be quoted: %s", arg)
		}
	}

	pattern := value
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimPrefix(strings.TrimSuffix(pattern, "/"), "/")
	} else {
		pattern = regexp.QuoteMeta(pattern)
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %v", err)
	}

	return &Filter{field: FilterField(fieldStr), re: re, negate: negate}, nil
}

// IsMatch checks if a note matches this filter
func (f *Filter) IsMatch(n *Metadata) bool {
	result := false
	switch f.field {
	case FilterDate:
		result = f.re.MatchString(n.Identifier)
	case FilterTitle:
		result = f.re.MatchString(n.Title)
	case FilterTag:
		result = slices.ContainsFunc(n.Tags, func(kw string) bool {
			return f.re.MatchString(kw)
		})
	case FilterAny: // any field
		if f.re.MatchString(n.Identifier) {
			result = true
		} else if f.re.MatchString(n.Title) {
			result = true
		} else {
			result = slices.ContainsFunc(n.Tags, func(kw string) bool {
				return f.re.MatchString(kw)
			})
		}
	default:
		return false
	}
	if f.negate {
		return !result
	}
	return result
}
