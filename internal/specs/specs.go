// Package specs checks the documentation layer for internal consistency.
//
// The layer is a graph: docs/ARCHITECTURE.yaml names one SRD per component and
// interface, each SRD carries requirements its acceptance criteria trace to,
// and docs/road-map.yaml assigns every SRD to a release. Nothing enforces
// those edges but this package, so a pointer to a file nobody wrote, a
// specification no component claims, a criterion tracing to a requirement
// that was renumbered, or a component the roadmap never schedules all survive
// review by looking plausible.
//
// The checks are ordinary functions returning a Report, so mage audit and
// go test both run them and neither needs the other installed.
package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Finding is one thing wrong with the documentation layer.
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

// Report is what a check run found. Findings are failures; notes are what the
// run observed and had no opinion about, which is where requirement coverage
// lands until there is an implementation to cover it.
type Report struct {
	Findings []Finding
	Notes    []string

	SRDFiles     int
	Pointers     int
	Releases     int
	Requirements int
	Covered      int
	Uncovered    []string
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
	return fmt.Errorf("%d documentation-layer %s:\n%s",
		len(r.Findings), plural(len(r.Findings), "problem"), strings.Join(lines, "\n"))
}

// Summary is the one-paragraph account of a run, for a human reading output
// rather than a test failure.
func (r Report) Summary() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "docs: %d SRDs, %d pointers, %d releases, %d requirements\n",
		r.SRDFiles, r.Pointers, r.Releases, r.Requirements)
	fmt.Fprintf(&builder, "coverage: %d of %d requirements referenced by a test\n",
		r.Covered, r.Requirements)
	for _, note := range r.Notes {
		fmt.Fprintf(&builder, "note: %s\n", note)
	}
	return builder.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// architecture is the part of ARCHITECTURE.yaml this package reads. Components
// and interfaces are the same shape here because both carry an SRD pointer,
// which is the only field the edge check needs.
type architecture struct {
	Components []pointerHolder `yaml:"components"`
	Interfaces []pointerHolder `yaml:"interfaces"`
}

type pointerHolder struct {
	Name string `yaml:"name"`
	SRD  string `yaml:"srd"`
}

// srd is the part of an SRD this package reads.
type srd struct {
	ID           string                  `yaml:"id"`
	Requirements map[string]requirements `yaml:"requirements"`
	Criteria     []criterion             `yaml:"acceptance_criteria"`
}

type requirements struct {
	Title string              `yaml:"title"`
	Items []map[string]string `yaml:"items"`
}

type criterion struct {
	ID     string   `yaml:"id"`
	Traces []string `yaml:"traces"`
}

// requirementReference matches the way a test names a requirement: the SRD id
// and the requirement id adjacent, as in "srd-3-escaping R1.1". Go test names
// cannot hold dots or hyphens, so in practice the reference sits in a comment
// or in a table-driven case's name field.
var requirementReference = regexp.MustCompile(`(srd-\d+-[a-z0-9-]+)[\s/_:-]+([Rr]\d+\.\d+)`)

// checkerPackage is skipped by the coverage scan. This package's own tests
// build documentation layers out of fixtures whose requirement ids exist only
// in that test data, and reading them as claims about the real layer reports
// every fixture as a test naming a requirement that does not exist.
const checkerPackage = "internal/specs"

// Check runs every documentation-layer check against the repository rooted at
// root and returns what it found. It returns an error only when the tree
// cannot be read at all; everything it is checking for comes back as a
// finding, so one run reports every problem rather than the first.
func Check(root string) (Report, error) {
	var report Report

	docs := filepath.Join(root, "docs")
	architecturePath := filepath.Join(docs, "ARCHITECTURE.yaml")

	for _, name := range []string{"VISION.yaml", "ARCHITECTURE.yaml"} {
		if finding := parseStrict(filepath.Join(docs, name)); finding != nil {
			report.Findings = append(report.Findings, *finding)
		}
	}

	srdFiles, err := filepath.Glob(filepath.Join(docs, "srd", "*.yaml"))
	if err != nil {
		return report, fmt.Errorf("read docs/srd: %w", err)
	}
	sort.Strings(srdFiles)
	report.SRDFiles = len(srdFiles)

	specifications := make(map[string]srd, len(srdFiles))
	for _, path := range srdFiles {
		if finding := parseStrict(path); finding != nil {
			report.Findings = append(report.Findings, *finding)
			continue
		}
		var parsed srd
		if err := decodeFile(path, &parsed); err != nil {
			report.Findings = append(report.Findings, Finding{relative(root, path), err.Error()})
			continue
		}
		specifications[path] = parsed
		report.Findings = append(report.Findings, checkSRD(root, path, parsed)...)
	}

	pointers, findings := readPointers(root, architecturePath)
	report.Findings = append(report.Findings, findings...)
	report.Pointers = len(pointers)
	report.Findings = append(report.Findings, checkEdges(root, docs, pointers, srdFiles)...)

	releases, findings := checkRoadMap(root, srdFiles)
	report.Findings = append(report.Findings, findings...)
	report.Releases = releases

	requirementIDs := collect(specifications)
	report.Requirements = len(requirementIDs)

	covered, findings := checkCoverage(root, requirementIDs)
	report.Findings = append(report.Findings, findings...)
	report.Covered = len(covered)
	for id := range requirementIDs {
		if !covered[id] {
			report.Uncovered = append(report.Uncovered, id)
		}
	}
	sort.Strings(report.Uncovered)
	if len(report.Uncovered) > 0 {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"no test yet names %d of the %d %s; each becomes a failure to fix once its component is implemented",
			len(report.Uncovered), report.Requirements, plural(report.Requirements, "requirement")))
	}

	return report, nil
}

// parseStrict reports whether a file parses as YAML with no duplicate mapping
// keys. Decoding into a map is what rejects a duplicate: a duplicate key
// decoded into a yaml.Node is accepted and silently keeps both.
func parseStrict(path string) *Finding {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Finding{filepath.Base(path), err.Error()}
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return &Finding{filepath.Base(path), err.Error()}
	}
	return nil
}

func decodeFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}

// readPointers returns every SRD path ARCHITECTURE names, from both components
// and interfaces. A component with no pointer is a finding: an unspecified
// component is the state this package exists to make visible.
func readPointers(root, path string) (map[string][]string, []Finding) {
	pointers := make(map[string][]string)
	var findings []Finding

	var parsed architecture
	if err := decodeFile(path, &parsed); err != nil {
		return pointers, append(findings, Finding{relative(root, path), err.Error()})
	}

	for _, group := range []struct {
		kind    string
		holders []pointerHolder
	}{
		{"component", parsed.Components},
		{"interface", parsed.Interfaces},
	} {
		for _, holder := range group.holders {
			if holder.SRD == "" {
				findings = append(findings, Finding{
					relative(root, path),
					fmt.Sprintf("%s %q names no srd", group.kind, holder.Name),
				})
				continue
			}
			pointers[holder.SRD] = append(pointers[holder.SRD], holder.Name)
		}
	}
	return pointers, findings
}

// checkEdges walks the architecture-to-SRD edge in both directions: a pointer
// with no file is a specification someone forgot to write, and a file no
// pointer names is one the architecture forgot to claim.
func checkEdges(root, docs string, pointers map[string][]string, srdFiles []string) []Finding {
	var findings []Finding

	named := make(map[string]bool, len(pointers))
	for pointer, holders := range pointers {
		target := filepath.Join(root, filepath.FromSlash(pointer))
		named[target] = true
		if _, err := os.Stat(target); err != nil {
			sort.Strings(holders)
			findings = append(findings, Finding{
				"docs/ARCHITECTURE.yaml",
				fmt.Sprintf("%s is named by %s but does not exist",
					pointer, strings.Join(holders, ", ")),
			})
		}
	}

	for _, path := range srdFiles {
		if filepath.Base(path) == "README.md" || named[path] {
			continue
		}
		findings = append(findings, Finding{
			relative(root, path),
			"no component or interface in ARCHITECTURE.yaml names this file",
		})
	}

	sortFindings(findings)
	return findings
}

// roadMap is the part of docs/road-map.yaml this package reads. A unit is an
// SRD id, which is also the file stem under docs/srd.
type roadMap struct {
	Releases []release `yaml:"releases"`
}

type release struct {
	ID    string   `yaml:"id"`
	Units []string `yaml:"units"`
}

// checkRoadMap walks the release-to-SRD edge in both directions: a unit
// naming no SRD on disk is scheduled work nobody specified, and an SRD no
// release claims never gets scheduled. Exactly one release per SRD, because a
// component in two releases has two chances to be called done and an
// implementer following the roadmap cannot tell which one binds.
func checkRoadMap(root string, srdFiles []string) (int, []Finding) {
	path := filepath.Join(root, "docs", "road-map.yaml")
	name := "docs/road-map.yaml"

	if finding := parseStrict(path); finding != nil {
		return 0, []Finding{*finding}
	}
	var parsed roadMap
	if err := decodeFile(path, &parsed); err != nil {
		return 0, []Finding{{name, err.Error()}}
	}

	stems := make(map[string]bool, len(srdFiles))
	for _, file := range srdFiles {
		stems[strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))] = true
	}

	var findings []Finding
	assigned := make(map[string][]string)
	for _, entry := range parsed.Releases {
		for _, unit := range entry.Units {
			if !stems[unit] {
				findings = append(findings, Finding{name,
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
			findings = append(findings, Finding{name,
				fmt.Sprintf("%s is assigned to no release", stem)})
		default:
			sort.Strings(releases)
			findings = append(findings, Finding{name,
				fmt.Sprintf("%s is assigned to %s; exactly one release owns an SRD",
					stem, strings.Join(releases, " and "))})
		}
	}

	sortFindings(findings)
	return len(parsed.Releases), findings
}

// checkSRD holds one specification to its own shape: the id matches the file
// it is in, and every acceptance criterion traces to a requirement that
// exists. A renumbered requirement leaves a criterion pointing at nothing, and
// nothing else notices.
func checkSRD(root, path string, parsed srd) []Finding {
	var findings []Finding
	name := relative(root, path)

	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if parsed.ID != stem {
		findings = append(findings, Finding{name,
			fmt.Sprintf("id %q does not match the file name", parsed.ID)})
	}

	items := requirementItems(parsed)
	if len(items) == 0 {
		findings = append(findings, Finding{name, "states no requirements"})
	}

	for _, criterion := range parsed.Criteria {
		for _, trace := range criterion.Traces {
			if !items[trace] {
				findings = append(findings, Finding{name,
					fmt.Sprintf("criterion %s traces to %s, which is not a requirement here",
						criterion.ID, trace)})
			}
		}
	}
	return findings
}

// requirementItems returns the requirement ids one SRD states. An item is a
// single-entry mapping from the id to its text, which is what keeps the ids
// visible in the YAML rather than buried in a field.
func requirementItems(parsed srd) map[string]bool {
	items := make(map[string]bool)
	for _, group := range parsed.Requirements {
		for _, item := range group.Items {
			for id := range item {
				items[id] = true
			}
		}
	}
	return items
}

// collect returns every requirement in the layer, qualified by its SRD id so
// two specifications numbering an R1.1 stay distinct.
func collect(specifications map[string]srd) map[string]bool {
	all := make(map[string]bool)
	for _, parsed := range specifications {
		for id := range requirementItems(parsed) {
			all[parsed.ID+" "+id] = true
		}
	}
	return all
}

// checkCoverage reads every test in the tree for requirement references and
// reports which requirements they name. A reference to a requirement that does
// not exist is a finding, because it means a test claims coverage it does not
// have. A requirement no test names is not: most of them have no
// implementation yet, and failing on that would fail the tree this check is
// meant to certify.
func checkCoverage(root string, requirements map[string]bool) (map[string]bool, []Finding) {
	covered := make(map[string]bool)
	var findings []Finding

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || relative(root, path) == checkerPackage {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range requirementReference.FindAllSubmatch(data, -1) {
			reference := string(match[1]) + " " + strings.ToUpper(string(match[2][:1])) + string(match[2][1:])
			if requirements[reference] {
				covered[reference] = true
				continue
			}
			findings = append(findings, Finding{relative(root, path),
				fmt.Sprintf("names %s, which is not a requirement in the docs layer", reference)})
		}
		return nil
	})
	if err != nil {
		findings = append(findings, Finding{"", fmt.Sprintf("read tests: %v", err)})
	}

	sortFindings(findings)
	return covered, findings
}

func relative(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Detail < findings[j].Detail
	})
}
