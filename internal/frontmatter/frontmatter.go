// Package frontmatter renders a paper's title page into the fragment IEEEtran
// expects.
//
// The title page is metadata with destinations rather than prose with a flow:
// the title belongs in a title command, the author in an author command, the
// abstract in an abstract environment. Converted as a chapter it comes out as
// body text under a heading, which is what the retired build did
// (srd001-front-matter).
package frontmatter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/petar-djukic/md-to-tex/internal/latex"
)

// Page is what a title page carries: metadata in its YAML frontmatter, and an
// abstract in one of the two places the manuscripts put it
// (srd001-front-matter R1.2, R1.6, R1.7).
type Page struct {
	Title    string `yaml:"title"`
	Subtitle string `yaml:"subtitle"`
	Date     string `yaml:"date"`
	Author   Author `yaml:"author"`

	// Abstract is the frontmatter field when the page states one, otherwise
	// the body below the abstract marker.
	Abstract string `yaml:"abstract"`

	// Body is what remains below the frontmatter once the abstract is
	// accounted for. A page whose abstract is a frontmatter field can still
	// carry front matter in its body -- a keywords block, typically -- which
	// is emitted after the abstract.
	Body string `yaml:"-"`
}

// Author is a title page's author field, which the manuscripts write either as
// a single scalar or as a list of names. Both decode; a list joins with the
// word and, which is what LaTeX expects between authors
// (srd001-front-matter R1.5).
type Author string

// UnmarshalYAML decodes either form.
func (a *Author) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		*a = Author(strings.Join(names, " and "))
		return nil
	}
	var name string
	if err := value.Decode(&name); err != nil {
		return err
	}
	*a = Author(name)
	return nil
}

var (
	// The content section is optional: Obsidian leaves an empty block behind
	// when the last property is removed, and a block that did not match here
	// would reach the parser as a thematic break.
	frontmatterBlock = regexp.MustCompile(`(?s)\A---\r?\n((?:.*?\r?\n)?)---\r?\n`)
	// The title page marks its abstract with a bold-italic run rather than a
	// heading, so it does not become a numbered section
	// (srd001-front-matter R1.7).
	abstractMarker = regexp.MustCompile(`(?m)^\s*\*{2,3}Abstract\*{2,3}\s*$`)
	// The author is written as a name followed by a mailto link
	// (srd001-front-matter R3.1).
	authorEmail = regexp.MustCompile(`\[([^\]]+)\]\(mailto:[^)]+\)`)
)

// Split separates a leading YAML frontmatter block from the body below it.
//
// It reports the block's own bytes, the body, and how far into the source the
// body begins, so a caller that renders the body can keep the source's line
// numbers: an error at the twelfth line of a file is reported at the twelfth
// line, not at the fifth line of what remains after the block is removed.
//
// Both paths through the library read this construct. The title page decodes
// the block as the metadata it renders from (srd001-front-matter R1.1), and a
// chapter drops it (srd002-renderer-core R6.6); one reader serves both, so the
// two cannot disagree about where a block ends.
func Split(source []byte) (block, body []byte, offset int, found bool) {
	match := frontmatterBlock.FindSubmatchIndex(source)
	if match == nil {
		return nil, source, 0, false
	}
	return source[match[2]:match[3]], source[match[1]:], match[1], true
}

// Read parses a title page. The name appears in error messages and nowhere
// else; Read opens no file (srd001-front-matter R1.1).
func Read(source []byte, name string) (Page, error) {
	var page Page

	block, body, _, found := Split(source)
	if !found {
		return page, fmt.Errorf("%s: has no YAML frontmatter", name)
	}

	// Decoding into a map is what rejects a duplicate key; decoding straight
	// into the struct would keep the last value silently
	// (srd001-front-matter R1.3).
	var fields map[string]any
	if err := yaml.Unmarshal(block, &fields); err != nil {
		return page, fmt.Errorf("%s frontmatter: %w", name, err)
	}
	if err := yaml.Unmarshal(block, &page); err != nil {
		return page, fmt.Errorf("%s frontmatter: %w", name, err)
	}
	if strings.TrimSpace(page.Title) == "" {
		return page, fmt.Errorf("%s frontmatter: states no title", name)
	}

	if strings.TrimSpace(page.Abstract) != "" {
		// The abstract came from the frontmatter, so whatever is in the body
		// is something else and belongs after it (srd001-front-matter R1.6).
		page.Body = strings.TrimSpace(string(body))
		return page, nil
	}
	if marker := abstractMarker.FindIndex(body); marker != nil {
		body = body[marker[1]:]
	}
	page.Abstract = strings.TrimSpace(string(body))
	return page, nil
}

// titleBlock is the fixed shape of the title, author, and maketitle commands.
// Every field substituted into it is escaped first, and the template escapes
// nothing itself (srd001-front-matter R2.4, srd003-escaping R3.3).
var titleBlock = template.Must(template.New("title").Parse(
	`\title{{"{"}}{{.Title}}{{"}"}}
{{if .Author}}\author{{"{"}}{{.Author}}{{"}"}}
{{end}}\maketitle
`))

// TitleBlock renders the title, author, and maketitle commands
// (srd001-front-matter R2.1, R2.2).
//
// The subtitle joins the title rather than taking a command of its own:
// IEEEtran has no subtitle command, and a second title line lands in the
// author block. The date is decoded and not emitted, because the title block
// has no date slot (srd001-front-matter R2.3).
func (p Page) TitleBlock() (string, error) {
	title := latex.Escape(p.Title)
	if subtitle := strings.TrimSpace(p.Subtitle); subtitle != "" {
		title += `\\ \large ` + latex.Escape(subtitle)
	}

	var rendered bytes.Buffer
	err := titleBlock.Execute(&rendered, struct{ Title, Author string }{
		Title:  title,
		Author: p.AuthorField(),
	})
	return rendered.String(), err
}

// AuthorField turns the author line into IEEEtran's author field, rendering a
// mailto link as a plain address rather than passing the markdown through
// (srd001-front-matter R3.1, R3.2, R3.3, R3.4).
func (p Page) AuthorField() string {
	author := strings.TrimSpace(string(p.Author))
	if author == "" {
		return ""
	}

	email := ""
	if match := authorEmail.FindStringSubmatch(author); match != nil {
		email = match[1]
		author = strings.TrimSpace(authorEmail.ReplaceAllString(author, ""))
	}

	field := latex.Escape(author)
	if email != "" {
		field += `\\ \texttt{` + latex.Escape(email) + `}`
	}
	return field
}
