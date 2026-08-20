# md-to-tex SRDs -- the requirements of record

An SRD is the contract one component answers to. It states what the component
must do in numbered requirements an implementation issue executes and a test
checks, and it is written before any of that component's code exists.
[ARCHITECTURE.yaml](../ARCHITECTURE.yaml) names the SRD that governs each
component; the audit target checks that pointer in both directions, so a
component without an SRD and an SRD no component names are both errors.

## Table 1: the SRD fields

| Field | Holds |
|-------|-------|
| `id` | Matches the file name without its extension, and the `srd:` pointer in ARCHITECTURE. |
| `title` | The component's name as ARCHITECTURE writes it. |
| `problem` | What goes wrong without this component, in enough detail to judge a requirement against it. |
| `goals` | G1, G2, ... -- what the component achieves, one line each. |
| `requirements` | Groups R1, R2, ... each with a `title` and `items` numbered R1.1, R1.2, ... |
| `examples` | Under a requirement group: `markdown` and `latex` pairs holding literal input and exact expected output. |
| `non_goals` | What the component does not do, so a reader stops looking for it here. |
| `acceptance_criteria` | Objects with `id`, `criterion`, and `traces` naming the requirement ids each covers. |

## How a requirement is written

A requirement is checkable or it is not a requirement. Every item states an
input and an output, or an invariant a test can assert. Prose intent -- "the
renderer should handle tables well" -- belongs in `problem`, not in `items`.

Where a requirement encodes a correspondence between the two formats, the
group carries an `examples` list holding the literal markdown and the exact
LaTeX it produces, whitespace included. Those pairs are fixtures: the
implementation tests read them as the expected output, and a disagreement
between the SRD and the code is a failing test rather than a discussion.

## How implementation traces back

An implementation issue names the SRD it executes and the requirement ids it
covers. Tests carry the same ids in their names or in a comment, written as the
SRD id and the requirement id adjacent -- `srd-3-escaping R1.1` -- since a Go
test name holds neither dots nor hyphens and a comment does. Several
requirements of one SRD may follow a single mention of it, separated by commas
or the word and: `srd-3-escaping R2.1, R2.3, and R3.1` credits all three. `mage audit` reads
every test for those references and reports which requirements none of them
names. A reference to a requirement that does not exist fails the audit, because
a test claiming coverage it does not have is worse than one claiming none.

A requirement that changes after implementation changes here first, and the
issue that changes it says what it breaks.
