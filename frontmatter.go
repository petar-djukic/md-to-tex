package mdtotex

import (
	"strings"

	"github.com/petar-djukic/md-to-tex/internal/frontmatter"
	"github.com/petar-djukic/md-to-tex/internal/render"
)

// RenderFrontMatter renders a paper's title page as the front-matter fragment
// the container inputs.
//
// The metadata reaches the commands IEEEtran expects through templates and the
// escaper; the abstract and whatever the page carries below it -- a keywords
// block, typically -- convert through the chapter path, so an author's
// emphasis, citations, and raw LaTeX survive (srd001-front-matter R4.1, R4.3).
//
// The name appears in error messages and nowhere else; no file is opened.
func RenderFrontMatter(source []byte, name string, options Options) (Result, error) {
	page, err := frontmatter.Read(source, name)
	if err != nil {
		return Result{}, err
	}

	title, err := page.TitleBlock()
	if err != nil {
		return Result{}, err
	}

	var fragment strings.Builder
	fragment.WriteString(title)

	// The order is fixed: title block, abstract, body
	// (srd001-front-matter R4.4).
	abstract, _, err := render.Convert([]byte(page.Abstract), name, options.renderConfig())
	if err != nil {
		return Result{}, err
	}
	if trimmed := strings.TrimSpace(string(abstract)); trimmed != "" {
		fragment.WriteString("\\begin{abstract}\n" + trimmed + "\n\\end{abstract}\n")
	}

	body, _, err := render.Convert([]byte(page.Body), name, options.renderConfig())
	if err != nil {
		return Result{}, err
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		fragment.WriteString(trimmed + "\n")
	}

	return Result{Name: name, LaTeX: []byte(fragment.String())}, nil
}
