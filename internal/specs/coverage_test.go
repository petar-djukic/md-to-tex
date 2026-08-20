package specs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpus writes a repository whose SRD states two requirements and whose suite
// claims what the caller asks it to claim.
type corpus struct {
	criteriaTraces []string
	caseTraces     []string
	caseGoTest     string
	releaseStatus  []string
}

func writeCorpus(t *testing.T, layout corpus) string {
	t.Helper()
	root := t.TempDir()

	requirements := filepath.Join(root, "docs", "specs", "software-requirements")
	suites := filepath.Join(root, "docs", "specs", "test-suites")
	for _, directory := range []string{requirements, suites} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	document := "id: srd001-alpha\ntitle: Alpha\nrequirements:\n  R1:\n    title: A group\n    items:\n" +
		"      - R1.1: The first requirement.\n      - R1.2: The second requirement.\n" +
		"acceptance_criteria:\n  - id: AC1\n    criterion: A criterion.\n    traces: [" +
		strings.Join(layout.criteriaTraces, ", ") + "]\n"
	write(t, filepath.Join(requirements, "srd001-alpha.yaml"), document)

	suite := "id: test-rel00.1-alpha\ntitle: A Suite\nrelease: \"00.1\"\ntest_cases:\n" +
		"  - name: 'a case'\n    use_case: rel00.1-uc001-alpha\n"
	if layout.caseGoTest != "" {
		suite += "    go_test: " + layout.caseGoTest + "\n"
	}
	suite += "    status: done\n    traces:\n"
	for _, trace := range layout.caseTraces {
		suite += "      - " + trace + "\n"
	}
	write(t, filepath.Join(suites, "test-rel00.1-alpha.yaml"), suite)

	architecture := "id: architecture-test\ntitle: Test\ncomponents:\n" +
		"  - name: Alpha\n    srd: docs/specs/software-requirements/srd001-alpha.yaml\n"
	write(t, filepath.Join(root, "docs", "ARCHITECTURE.yaml"), architecture)

	roadMap := "releases:\n"
	for i, status := range layout.releaseStatus {
		roadMap += "  - id: rel0" + string(rune('0'+i)) + ".0\n    status: " + status +
			"\n    units:\n"
		if i == 0 {
			roadMap += "      - srd001-alpha\n"
		}
	}
	write(t, filepath.Join(root, "docs", "road-map.yaml"), roadMap)
	return root
}

// TestCoverageGatesOnlyWhenImplementationIsDone covers the rule the roadmap
// statuses decide: a requirement nothing tests is a note while a release is
// still pending, and a finding once every release but the last is complete.
func TestCoverageGatesOnlyWhenImplementationIsDone(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		gated    bool
	}{
		{"still building", []string{"complete", "pending", "pending"}, false},
		{"the last release open", []string{"complete", "complete", "pending"}, true},
		{"everything complete", []string{"complete", "complete", "complete"}, true},
		{"one release only", []string{"pending"}, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeCorpus(t, corpus{
				criteriaTraces: []string{"R1.1"},
				caseTraces:     []string{"srd001-alpha R1.1"},
				caseGoTest:     "TestAlpha",
				releaseStatus:  testCase.statuses,
			})

			report, err := Check(root)
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}
			if report.Coverage.Gated != testCase.gated {
				t.Errorf("Gated = %v, want %v", report.Coverage.Gated, testCase.gated)
			}
			if report.Coverage.Covered != 1 || report.Coverage.Requirements != 2 {
				t.Errorf("coverage = %d of %d, want 1 of 2",
					report.Coverage.Covered, report.Coverage.Requirements)
			}

			problems := report.Err()
			named := problems != nil && strings.Contains(problems.Error(), "srd001-alpha R1.2")
			if named != testCase.gated {
				t.Errorf("the uncovered requirement reported as a finding = %v, want %v: %v",
					named, testCase.gated, problems)
			}
		})
	}
}

// TestCoverageFollowsACriterion covers the transitive edge: a case that claims
// a criterion claims the requirements that criterion asserts, which is how the
// corpus states evidence.
func TestCoverageFollowsACriterion(t *testing.T) {
	root := writeCorpus(t, corpus{
		criteriaTraces: []string{"R1.1", "R1.2"},
		caseTraces:     []string{"srd001-alpha AC1"},
		caseGoTest:     "TestAlpha",
		releaseStatus:  []string{"complete", "pending"},
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if report.Coverage.Covered != 2 {
		t.Errorf("Covered = %d, want both requirements through the criterion", report.Coverage.Covered)
	}
	if problems := report.Err(); problems != nil {
		t.Fatalf("a fully covered corpus reported:\n%v", problems)
	}
}

// TestAPlannedCaseClaimsNothing covers the rule that intent is not evidence: a
// case naming no Go test leaves its requirements uncovered.
func TestAPlannedCaseClaimsNothing(t *testing.T) {
	root := writeCorpus(t, corpus{
		criteriaTraces: []string{"R1.1", "R1.2"},
		caseTraces:     []string{"srd001-alpha AC1"},
		caseGoTest:     "",
		releaseStatus:  []string{"complete", "pending"},
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if report.Coverage.Covered != 0 {
		t.Errorf("Covered = %d, want none: a planned case names no test", report.Coverage.Covered)
	}
}

// TestSummaryReportsTheGate covers the line a human reads: the count, and
// whether an uncovered requirement would fail the run.
func TestSummaryReportsTheGate(t *testing.T) {
	root := writeCorpus(t, corpus{
		criteriaTraces: []string{"R1.1", "R1.2"},
		caseTraces:     []string{"srd001-alpha AC1"},
		caseGoTest:     "TestAlpha",
		releaseStatus:  []string{"complete", "complete"},
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	summary := report.Summary()
	if !strings.Contains(summary, "2 of 2 requirements named by a test case (gated)") {
		t.Errorf("Summary() = %q", summary)
	}
}

// TestCoverageOverThisRepository runs the gate against the corpus it governs,
// which is what mage audit dispatches to.
func TestCoverageOverThisRepository(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if !report.Coverage.Gated {
		t.Error("the roadmap says implementation is still open; the gate should be live by rel01.0")
	}
	if report.Coverage.Covered != report.Coverage.Requirements {
		t.Errorf("%d of %d requirements are named by a test case; uncovered: %v",
			report.Coverage.Covered, report.Coverage.Requirements, report.Coverage.Uncovered)
	}
}
