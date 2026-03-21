package frontmatter

import (
	"denotesrv/pkg/metadata"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Templates for denote frontmatter (org, md, txt)
var templates = map[metadata.FileType]string{
	metadata.FileTypeOrg: `#+title:      %s
#+date:       %s
#+filetags:   %s
#+identifier: %s
#+signature:  %s

`,
	metadata.FileTypeMdYaml: `---
title:      %s
date:       %s
tags:       %s
identifier: %s
signature:  %s
---

`,
	metadata.FileTypeMdToml: `+++
title      = %s
date       = %s
tags       = %s
identifier = %s
signature  = %s
+++

`,
	metadata.FileTypeTxt: `title:      %s
date:       %s
tags:       %s
identifier: %s
signature:  %s
---------------------------

`,
}

// formatTags formats tags according to file type
func formatTags(tags []string, fileType metadata.FileType) string {
	if len(tags) == 0 {
		return ""
	}
	switch fileType {
	case metadata.FileTypeOrg:
		return ":" + strings.Join(tags, ":") + ":"
	case metadata.FileTypeMdYaml, metadata.FileTypeMdToml:
		quoted := make([]string, len(tags))
		for i, t := range tags {
			quoted[i] = `"` + t + `"`
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	default:
		return strings.Join(tags, "  ")
	}
}

// Marshal returns the formatted frontmatter content as bytes
func Marshal(fm *metadata.FrontMatter, fileType metadata.FileType) []byte {
	template := templates[fileType]
	dateStr := time.Now().Format("2006-01-02 Mon 15:04")

	// For org-mode, wrap date in brackets for timestamp
	if fileType == metadata.FileTypeOrg {
		dateStr = "[" + dateStr + "]"
	}

	keywordsStr := formatTags(fm.Tags, fileType)
	content := fmt.Sprintf(template, fm.Title, dateStr, keywordsStr, fm.Identifier, fm.Signature)
	return []byte(content)
}

var tagSplitRe = regexp.MustCompile(`[\s,;:]+`)
var tagStripRe = regexp.MustCompile(`[^a-z0-9]`)

// parseTags parses a raw tag string into normalized tags.
// Accepts any delimiter format: "[tag1, tag2]", "tag1 tag2", ":tag1:tag2:".
// Normalizes each tag to [a-z0-9]+, dropping empty results.
func parseTags(s string) []string {
	s = strings.Trim(s, "[] \t")
	if s == "" {
		return nil
	}
	parts := tagSplitRe.Split(s, -1)
	var tags []string
	for _, p := range parts {
		p = tagStripRe.ReplaceAllString(strings.ToLower(p), "")
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

// Unmarshal extracts front matter from file content.
// ext should be the file extension (e.g., ".md", ".org", ".txt").
// Returns the parsed frontmatter and the detected FileType.
func Unmarshal(content []byte, ext string) (*metadata.FrontMatter, metadata.FileType, error) {
	ext = strings.ToLower(ext)
	text := string(content)

	fm := &metadata.FrontMatter{}
	var fileType metadata.FileType

	switch ext {
	case ".org":
		fileType = metadata.FileTypeOrg
		if m := regexp.MustCompile(`(?m)^#\+title:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Title = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`(?m)^#\+filetags:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Tags = parseTags(m[1])
		}
		if m := regexp.MustCompile(`(?m)^#\+identifier:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Identifier = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`(?m)^#\+signature:[ \t]*(.*)$`).FindStringSubmatch(text); m != nil {
			fm.Signature = strings.TrimSpace(m[1])
		}

	case ".md":
		// Try YAML first
		yamlRe := regexp.MustCompile(`(?ms)^---\n(.*?)\n---`)
		if m := yamlRe.FindStringSubmatch(text); m != nil {
			fileType = metadata.FileTypeMdYaml
			yamlContent := m[1]
			if m := regexp.MustCompile(`(?m)^title:[ \t]*["']?(.+?)["']?$`).FindStringSubmatch(yamlContent); m != nil {
				fm.Title = strings.TrimSpace(m[1])
			}
			if m := regexp.MustCompile(`(?m)^tags:[ \t]*(.+)$`).FindStringSubmatch(yamlContent); m != nil {
				fm.Tags = parseTags(m[1])
			}
			if m := regexp.MustCompile(`(?m)^identifier:[ \t]*["']?(.+?)["']?$`).FindStringSubmatch(yamlContent); m != nil {
				fm.Identifier = strings.TrimSpace(m[1])
			}
			if m := regexp.MustCompile(`(?m)^signature:[ \t]*["']?(.*)["']?$`).FindStringSubmatch(yamlContent); m != nil {
				fm.Signature = strings.TrimSpace(m[1])
			}
		} else {
			// Try TOML
			tomlRe := regexp.MustCompile(`(?ms)^\+\+\+\n(.*?)\n\+\+\+`)
			if m := tomlRe.FindStringSubmatch(text); m != nil {
				fileType = metadata.FileTypeMdToml
				tomlContent := m[1]
				if m := regexp.MustCompile(`(?m)^title[ \t]*=[ \t]*["']?(.+?)["']?$`).FindStringSubmatch(tomlContent); m != nil {
					fm.Title = strings.TrimSpace(m[1])
				}
				if m := regexp.MustCompile(`(?m)^tags[ \t]*=[ \t]*(.+)$`).FindStringSubmatch(tomlContent); m != nil {
					fm.Tags = parseTags(m[1])
				}
				if m := regexp.MustCompile(`(?m)^identifier[ \t]*=[ \t]*["']?(.+?)["']?$`).FindStringSubmatch(tomlContent); m != nil {
					fm.Identifier = strings.TrimSpace(m[1])
				}
				if m := regexp.MustCompile(`(?m)^signature[ \t]*=[ \t]*["']?(.*)["']?$`).FindStringSubmatch(tomlContent); m != nil {
					fm.Signature = strings.TrimSpace(m[1])
				}
			}
		}

	case ".txt":
		fileType = metadata.FileTypeTxt
		if m := regexp.MustCompile(`(?m)^title:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Title = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`(?m)^tags:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Tags = parseTags(m[1])
		}
		if m := regexp.MustCompile(`(?m)^identifier:[ \t]*(.+)$`).FindStringSubmatch(text); m != nil {
			fm.Identifier = strings.TrimSpace(m[1])
		}
		if m := regexp.MustCompile(`(?m)^signature:[ \t]*(.*)$`).FindStringSubmatch(text); m != nil {
			fm.Signature = strings.TrimSpace(m[1])
		}
	}

	return fm, fileType, nil
}
