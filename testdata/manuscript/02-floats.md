# Floats in two columns {#sec:floats}

A float carries its caption once, and the *emitted* form stays plain so an
edit made in the LaTeX comes back as prose.

| Construct | Written as | Renders to |
|-----------|------------|------------|
| Figure | an image block with an identifier | a figure float carrying caption and label |
| Table | a pipe table and a caption line | a tabularx float at the measure its content needs |

Table: What the float renderers map, and the markdown each construct is written as. {#tab:floats}

![A diagram spanning both columns.](fig/02-wide.pdf){#fig:wide .wide}
