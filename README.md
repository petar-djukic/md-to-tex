# md-to-tex

Go library that converts Obsidian-native markdown manuscripts into IEEEtran
LaTeX.

Paper pipelines that keep markdown as the source of truth route conversion
through pandoc, which owns the mapping between markdown constructs and the
LaTeX a journal class needs, and that ownership makes the mapping hard to
constrain: captions, file names, and float forms come out however pandoc and
its filters decide. md-to-tex moves the mapping into Go code the pipeline owns.
Chapter and figure file names match one-to-one across formats, a caption is
written once — image alt text, or a table caption line — and renders to plain
`\caption{}`/`\label{}` forms, and the emitted LaTeX stays close enough to
standard forms that edits made on the tex side diff back into markdown as
prose. Front matter renders from YAML metadata by template and LaTeX escaping;
chapters render through a goldmark AST walk with extensions for citations and
raw LaTeX passthrough.

The library is consumed by the paperkit build in
[autogenic-systems](https://github.com/petar-djukic/autogenic-systems), which
keeps pandoc for the tex-to-markdown backport direction and for PDF
compilation via latexmk.

## Status

The library is complete through rel01.0: every stated requirement is named
by a passing test, and the fixture manuscript under `testdata/` converts end
to end against committed expectations. rel02.0 adds the standalone service:
`md-to-tex serve` answers `POST /v1/parse` (front matter, headings, segments
with rune offsets, citations, links, problems), `POST /v1/convert` (the
library's result), `POST /v1/render` (a PDF compiled under latexmk), and
`GET /healthz`. The `Dockerfile` builds it on `texlive/texlive:latest-small`
with IEEEtran added; `mage image` builds the image and `mage imageSmoke`
renders the fixture manuscript through it. The first consumer is the corpus
importer in
[agentic-wiki-mesh](https://github.com/petar-djukic/agentic-wiki-mesh).
