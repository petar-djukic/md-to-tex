package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// suiteSubdir is where the shared corpus format keeps test suites.
const suiteSubdir = "docs/specs/test-suites"

// requirementDocument is the part of an SRD the coverage walk reads.
type requirementDocument struct {
	ID           string                      `yaml:"id"`
	Requirements map[string]requirementGroup `yaml:"requirements"`
	Criteria     []acceptanceCriterion       `yaml:"acceptance_criteria"`
}

type requirementGroup struct {
	Items []map[string]string `yaml:"items"`
}

type acceptanceCriterion struct {
	ID     string   `yaml:"id"`
	Traces []string `yaml:"traces"`
}

// testSuite is the part of a suite the coverage walk reads.
type testSuite struct {
	ID    string     `yaml:"id"`
	Cases []testCase `yaml:"test_cases"`
}

type testCase struct {
	Name   string   `yaml:"name"`
	GoTest string   `yaml:"go_test"`
	Status string   `yaml:"status"`
	Traces []string `yaml:"traces"`
}

// Coverage is what the requirement inventory looks like from the evidence
// side: which requirements some passing test claims, and which none does.
type Coverage struct {
	Requirements int
	Covered      int
	Uncovered    []string

	// Gated says whether an uncovered requirement is a finding. It becomes
	// true once the roadmap says implementation is done, which is the point at
	// which a requirement nothing tests is a gap rather than a schedule.
	Gated bool
}

// checkCoverage walks requirement to acceptance criterion to test case, and
// reports what no test claims.
//
// The edge is transitive because that is how the corpus states it: a test case
// traces the criteria and requirements it proves, and a criterion traces the
// requirements it asserts. A requirement is covered when some case names it,
// directly or through a criterion that names it.
//
// The specification-critic checks the same graph and reports an uncovered
// criterion as a warning, which is right while a repository is still building.
// This gate is the repository's own answer to the question the critic leaves
// open: once every release but the last is complete, a requirement nothing
// tests is a finding.
func checkCoverage(root string, srdFiles []string, releases []release) (Coverage, []Finding) {
	var coverage Coverage

	documents := make([]requirementDocument, 0, len(srdFiles))
	for _, path := range srdFiles {
		var parsed requirementDocument
		if err := decode(path, &parsed); err != nil {
			return coverage, []Finding{{relative(root, path), err.Error()}}
		}
		documents = append(documents, parsed)
	}

	claimed, findings := claimedByTests(root)
	if len(findings) > 0 {
		return coverage, findings
	}

	for _, document := range documents {
		// A criterion carries its requirements forward: a case that claims the
		// criterion claims what the criterion asserts.
		byCriterion := make(map[string][]string, len(document.Criteria))
		for _, criterion := range document.Criteria {
			id := document.ID + " " + criterion.ID
			for _, trace := range criterion.Traces {
				byCriterion[id] = append(byCriterion[id], document.ID+" "+trace)
			}
		}
		for criterion, requirements := range byCriterion {
			if claimed[criterion] {
				for _, requirement := range requirements {
					claimed[requirement] = true
				}
			}
		}

		for _, group := range document.Requirements {
			for _, item := range group.Items {
				for id := range item {
					requirement := document.ID + " " + id
					coverage.Requirements++
					if claimed[requirement] {
						coverage.Covered++
						continue
					}
					coverage.Uncovered = append(coverage.Uncovered, requirement)
				}
			}
		}
	}
	sort.Strings(coverage.Uncovered)

	coverage.Gated = implementationComplete(releases)
	if coverage.Gated {
		for _, requirement := range coverage.Uncovered {
			findings = append(findings, Finding{"docs/specs/test-suites",
				fmt.Sprintf("%s is named by no test case, and every release but the last is complete", requirement)})
		}
	}
	return coverage, findings
}

// claimedByTests reads the suites for what their cases claim. A case claims
// nothing until it names a Go test: a planned case records intent, and intent
// is not evidence.
func claimedByTests(root string) (map[string]bool, []Finding) {
	paths, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(suiteSubdir), "*.yaml"))
	if err != nil {
		return nil, []Finding{{suiteSubdir, err.Error()}}
	}

	claimed := make(map[string]bool)
	for _, path := range paths {
		var suite testSuite
		if err := decode(path, &suite); err != nil {
			return nil, []Finding{{relative(root, path), err.Error()}}
		}
		for _, testCase := range suite.Cases {
			if strings.TrimSpace(testCase.GoTest) == "" {
				continue
			}
			for _, trace := range testCase.Traces {
				claimed[trace] = true
			}
		}
	}
	return claimed, nil
}

// implementationComplete reports whether the roadmap says the work is done
// but for the last release, which is the release this gate belongs to.
//
// Reading the statuses the roadmap already carries is what keeps the gate from
// needing configuration of its own: a repository still building has releases
// pending, and the gate stays a note.
func implementationComplete(releases []release) bool {
	if len(releases) < 2 {
		return false
	}
	for _, entry := range releases[:len(releases)-1] {
		if entry.Status != "complete" {
			return false
		}
	}
	return true
}

func decode(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}
