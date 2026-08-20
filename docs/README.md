# md-to-tex docs -- the specification layer

The corpus follows the shared specification format the sibling repositories
use, so one tool validates all of them: requirement documents under
`docs/specs/software-requirements/`, use cases under `docs/specs/use-cases/`,
test suites under `docs/specs/test-suites/`, and `docs/SPECIFICATIONS.yaml`
indexing them. `mage audit` runs the specification-critic over that corpus.

Development here is specification-driven: the design exists as documents
before any component exists as code, and an implementation issue executes an
SRD rather than deriving one. The layer holds four kinds of file.
[VISION.yaml](VISION.yaml) states what the library is for, what it is not, and
how we will know it works. [ARCHITECTURE.yaml](ARCHITECTURE.yaml) decomposes
the library into components and records the decisions that fix the mapping
between markdown and LaTeX. The SRDs under
[specs/software-requirements/](specs/software-requirements/) carry the requirements
of each component, stated as a markdown input paired with the exact LaTeX
output wherever a requirement encodes a correspondence between the two
formats. [road-map.yaml](road-map.yaml) assigns every SRD to a release, so
implementation has an order and each release ships something the consuming
pipeline can adopt on its own.

## Table 1: the traceability edges

| Edge | From -> to | Checked by |
|------|-----------|------------|
| components | ARCHITECTURE `srd:` pointers -> requirement documents | `internal/specs`, both directions |
| requirements | the corpus graph: index, documents, and their references | the specification-critic |
| coverage | requirement items -> acceptance criteria -> test cases | the specification-critic |
| evidence | test case `go_test` -> a Go test that exists | the test-evidence audit |
| use cases | touchpoints -> requirement groups and criteria | the specification-critic |
| releases | road-map `units` -> requirement documents, exactly one release per document | `internal/specs`, both directions |

Every edge is walked in both directions. A pointer resolving to no file and a
document no pointer names are both errors, as are a criterion tracing a
requirement that does not exist and a test claim naming a Go test that does
not. Parsing is strict throughout, so a duplicate mapping key fails rather
than dropping a field.

## The commands

`mage audit` runs the corpus checks before the code checks. The corpus checks
are the specification-critic from
[declarative-agents](https://github.com/petar-djukic/declarative-agents), which
validates this same format across the sibling repositories: it is found beside
this repository, or through `AGENT_CORE_ROOT` and `AGENT_PROFILES_ROOT`, and
built from source when the checkout carries no binary. The release edge is
this repository's own, because the shared roadmap schema has no field for it,
and the component edge is ours because ARCHITECTURE.yaml is not part of the shared corpus; both run under `go test ./...` with no checkout present.

`mage specs` runs the corpus checks alone.