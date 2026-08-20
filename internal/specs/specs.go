// Package specs checks the two documentation-layer edges the shared corpus
// format has no place for.
//
// Most of what this package used to check now belongs to the
// specification-critic, which validates the same corpus across the sibling
// repositories: strict parsing of the corpus files, requirement items against
// acceptance criteria, use-case touchpoints, the specification index, and the
// test claims. This repository keeps no second implementation of any of that.
//
// Two documents sit outside the shared corpus and carry edges of their own.
// docs/ARCHITECTURE.yaml names the requirement document that governs each
// component, and the critic does not read it: a run passes with a pointer
// resolving to nothing. docs/road-map.yaml assigns each document to a release
// through a units field the shared schema has no place for. Both edges are
// walked here, in both directions.
package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// srdSubdir is where the shared corpus format keeps software requirements.
// The specification-critic loads them from here, so this path is not ours to
// choose (declarative-agents agent-core/pkg/spec/corpus.go).
const srdSubdir = "docs/specs/software-requirements"

// Finding is one thing wrong with the release-to-SRD edge.
type Finding struct {
	File   string
	Detail string
}

func (f Finding) String() string {
	if f.File == "" {
		return f.Detail
	}
	return f.File + ": " + f.Detail
}

// Report is what a check run found.
type Report struct {
	Findings []Finding

	SRDFiles int
	Pointers int
	Releases int
	Coverage Coverage
}

// Err reports the findings as one error, or nil when there are none.
func (r Report) Err() error {
	if len(r.Findings) == 0 {
		return nil
	}
	lines := make([]string, 0, len(r.Findings))
	for _, finding := range r.Findings {
		lines = append(lines, "  "+finding.String())
	}
	problems := "problems"
	if len(r.Findings) == 1 {
		problems = "problem"
	}
	return fmt.Errorf("%d documentation-layer %s:\n%s", len(r.Findings), problems, strings.Join(lines, "\n"))
}

// Summary is the one-line account of a run, for a human reading output rather
// than a test failure.
func (r Report) Summary() string {
	gate := "reported"
	if r.Coverage.Gated {
		gate = "gated"
	}
	return fmt.Sprintf("architecture: %d pointers; road map: %d releases over %d requirement documents\n"+
		"coverage: %d of %d requirements named by a test case (%s)\n",
		r.Pointers, r.Releases, r.SRDFiles,
		r.Coverage.Covered, r.Coverage.Requirements, gate)
}

// architecture is the part of ARCHITECTURE.yaml this package reads. Components
// and interfaces have the same shape here because both carry an SRD pointer,
// which is the only field the edge check needs.
type architecture struct {
	Components []pointerHolder `yaml:"components"`
	Interfaces []pointerHolder `yaml:"interfaces"`
}

type pointerHolder struct {
	Name string `yaml:"name"`
	SRD  string `yaml:"srd"`
}

type roadMap struct {
	Releases []release `yaml:"releases"`
}

type release struct {
	ID     string   `yaml:"id"`
	Status string   `yaml:"status"`
	Units  []string `yaml:"units"`
}

// Check walks the release-to-SRD edge in both directions for the repository
// rooted at root.
//
// A unit naming no SRD on disk is scheduled work nobody specified, and an SRD
// no release claims never gets scheduled. Exactly one release per SRD, because
// a component in two releases has two chances to be called done and an
// implementer following the roadmap cannot tell which one binds.
func Check(root string) (Report, error) {
	var report Report

	srdFiles, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(srdSubdir), "srd*.yaml"))
	if err != nil {
		return report, fmt.Errorf("read %s: %w", srdSubdir, err)
	}
	report.SRDFiles = len(srdFiles)

	pointers, findings := checkArchitecture(root, srdFiles)
	report.Pointers = pointers
	report.Findings = append(report.Findings, findings...)

	path := filepath.Join(root, "docs", "road-map.yaml")
	name := "docs/road-map.yaml"

	data, err := os.ReadFile(path)
	if err != nil {
		report.Findings = append(report.Findings, Finding{name, err.Error()})
		return report, nil
	}
	var parsed roadMap
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		report.Findings = append(report.Findings, Finding{name, err.Error()})
		return report, nil
	}
	report.Releases = len(parsed.Releases)

	stems := make(map[string]bool, len(srdFiles))
	for _, file := range srdFiles {
		stems[strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))] = true
	}

	assigned := make(map[string][]string)
	for _, entry := range parsed.Releases {
		for _, unit := range entry.Units {
			if !stems[unit] {
				report.Findings = append(report.Findings, Finding{name,
					fmt.Sprintf("release %s names %s, which is not an SRD on disk", entry.ID, unit)})
				continue
			}
			assigned[unit] = append(assigned[unit], entry.ID)
		}
	}
	for stem := range stems {
		switch releases := assigned[stem]; len(releases) {
		case 1:
		case 0:
			report.Findings = append(report.Findings, Finding{name,
				fmt.Sprintf("%s is assigned to no release", stem)})
		default:
			sort.Strings(releases)
			report.Findings = append(report.Findings, Finding{name,
				fmt.Sprintf("%s is assigned to %s; exactly one release owns an SRD",
					stem, strings.Join(releases, " and "))})
		}
	}

	coverage, coverageFindings := checkCoverage(root, srdFiles, parsed.Releases)
	report.Coverage = coverage
	report.Findings = append(report.Findings, coverageFindings...)

	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].File != report.Findings[j].File {
			return report.Findings[i].File < report.Findings[j].File
		}
		return report.Findings[i].Detail < report.Findings[j].Detail
	})
	return report, nil
}

// checkArchitecture walks the component-to-SRD edge in both directions. A
// pointer with no file is a specification someone forgot to write, and a file
// no pointer names is one the architecture forgot to claim.
//
// The critic cannot do this: ARCHITECTURE.yaml is this repository's own
// document rather than part of the shared corpus, and a run passes with the
// pointer dangling.
func checkArchitecture(root string, srdFiles []string) (int, []Finding) {
	path := filepath.Join(root, "docs", "ARCHITECTURE.yaml")
	name := "docs/ARCHITECTURE.yaml"

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, []Finding{{name, err.Error()}}
	}
	// Decoding into a map is what rejects a duplicate key; a struct decode
	// would accept the file and keep the last value silently.
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return 0, []Finding{{name, err.Error()}}
	}
	var parsed architecture
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return 0, []Finding{{name, err.Error()}}
	}

	var findings []Finding
	named := make(map[string]bool)
	pointers := 0
	for _, group := range []struct {
		kind    string
		holders []pointerHolder
	}{
		{"component", parsed.Components},
		{"interface", parsed.Interfaces},
	} {
		for _, holder := range group.holders {
			if holder.SRD == "" {
				findings = append(findings, Finding{name,
					fmt.Sprintf("%s %q names no srd", group.kind, holder.Name)})
				continue
			}
			pointers++
			target := filepath.Join(root, filepath.FromSlash(holder.SRD))
			named[target] = true
			if _, err := os.Stat(target); err != nil {
				findings = append(findings, Finding{name,
					fmt.Sprintf("%s %q names %s, which does not exist",
						group.kind, holder.Name, holder.SRD)})
			}
		}
	}

	for _, file := range srdFiles {
		if !named[file] {
			findings = append(findings, Finding{relative(root, file),
				"no component or interface in ARCHITECTURE.yaml names this file"})
		}
	}
	return pointers, findings
}

func relative(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
