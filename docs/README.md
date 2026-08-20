# md-to-tex docs -- the specification layer

Development here is specification-driven: the design exists as documents
before any component exists as code, and an implementation issue executes an
SRD rather than deriving one. The layer holds three kinds of file.
[VISION.yaml](VISION.yaml) states what the library is for, what it is not, and
how we will know it works. [ARCHITECTURE.yaml](ARCHITECTURE.yaml) decomposes
the library into components and records the decisions that fix the mapping
between markdown and LaTeX. The SRDs under [srd/](srd/) carry the requirements
of each component, stated as a markdown input paired with the exact LaTeX
output wherever a requirement encodes a correspondence between the two formats.

## Table 1: the traceability edges

| Edge | From -> to | Checked by |
|------|-----------|------------|
| goals | VISION success criteria -> ARCHITECTURE components | inspection |
| requirements | ARCHITECTURE `srd:` pointers -> `docs/srd/*.yaml` | `mage audit`, both directions |
| implementation | SRD requirement ids -> tests naming them | `mage audit` |

Both directions on requirements means every pointer resolves to a file and
every SRD on disk is named by a component, so neither a dangling pointer nor
an orphan specification survives the audit. The audit also parses every file
strictly, which makes a duplicate mapping key an error rather than a dropped
field.
