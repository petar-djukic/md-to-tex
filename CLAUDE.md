# md-to-tex

This repository holds the Go library that converts Obsidian-native markdown
manuscripts into IEEEtran LaTeX for the paper pipelines in
petar-djukic/autogenic-systems. Markdown is the source of truth: chapters,
figures, and tables are written as native markdown that Obsidian renders, and
the library maps them to the LaTeX the journal class needs. File names match
one-to-one across formats, captions are written once and translate to
`\caption{}`/`\label{}` in both directions, and front matter renders from YAML
metadata by template and escaping rather than through pandoc.

Development is specification-driven. The docs layer under `docs/` (VISION,
ARCHITECTURE, SRDs) is the source of requirements; read it before implementing,
and do not write implementation code for a component whose SRD does not exist.

All work goes through issues and pull requests; never commit to `main`
directly. The `.claude` directory is a relative symlink into
`../coding-skills/.claude`, which assumes this repository is cloned beside
petar-djukic/coding-skills; the rules and skills there govern the workflow.
