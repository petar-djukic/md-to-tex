# Introduction {#sec:intro}

<!-- S1 -- governed by docs/specs/software-requirements/srd002-renderer-core.yaml -->

A paper pipeline that keeps markdown as the source of truth has to translate
it into the LaTeX its venue expects. Routing that through a general converter
costs 100% of the control over R&D_notation and the file names
[@coronado-2022-ztn-survey; @du-2023].

## What the mapping fixes

The correspondence is written once and holds in both directions:

- chapter and figure file names match across formats
- a caption is written once, as alt text or a caption line
- the emitted LaTeX stays close to what an author hand-writes

Section \ref{sec:floats} shows the floats, and Figure \ref{fig:pipeline} the
shape of the conversion.

![The conversion path, from markdown through the walk to a LaTeX fragment.](fig/01-pipeline.pdf){#fig:pipeline}
