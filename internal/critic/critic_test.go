package critic

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// checkout builds a declarative-agents layout under dir and returns its root.
func checkout(t *testing.T, dir string, withProfile bool) string {
	t.Helper()
	root := filepath.Join(dir, "declarative-agents")
	if err := os.MkdirAll(filepath.Join(root, "agent-core"), 0o755); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "applications", "catalog", "agents", "specification-critic")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if withProfile {
		for _, name := range []string{"profile.yaml", "audit-profile.yaml"} {
			if err := os.WriteFile(filepath.Join(agents, name), []byte("name: specification-critic\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func noEnv(string) string { return "" }

// TestResolveFindsTheSiblingCheckout covers the convention: with nothing set,
// a checkout beside the repository is found.
func TestResolveFindsTheSiblingCheckout(t *testing.T) {
	parent := t.TempDir()
	sibling := checkout(t, parent, true)
	repository := filepath.Join(parent, "md-to-tex")

	install, err := Resolve(repository, noEnv, os.Stat)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if install.CoreRoot != filepath.Join(sibling, "agent-core") {
		t.Errorf("CoreRoot = %q", install.CoreRoot)
	}
	if install.CatalogRoot != filepath.Join(sibling, "applications", "catalog") {
		t.Errorf("CatalogRoot = %q", install.CatalogRoot)
	}
}

// TestResolveHonoursTheEnvironmentOverrides covers the rule that an explicit
// setting wins, so a developer whose checkout sits elsewhere is not forced
// into a directory layout.
func TestResolveHonoursTheEnvironmentOverrides(t *testing.T) {
	parent := t.TempDir()
	checkout(t, parent, true) // the sibling that must not be chosen
	elsewhere := checkout(t, t.TempDir(), true)

	cases := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "both roots named directly",
			env: map[string]string{
				"AGENT_CORE_ROOT":     filepath.Join(elsewhere, "agent-core"),
				"AGENT_PROFILES_ROOT": filepath.Join(elsewhere, "applications", "catalog"),
			},
		},
		{
			name: "the checkout named once",
			env:  map[string]string{"DECLARATIVE_AGENTS_ROOT": elsewhere},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			getenv := func(key string) string { return testCase.env[key] }

			install, err := Resolve(filepath.Join(parent, "md-to-tex"), getenv, os.Stat)
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			if !strings.HasPrefix(install.CoreRoot, elsewhere) {
				t.Errorf("CoreRoot = %q, want it under the override at %q", install.CoreRoot, elsewhere)
			}
		})
	}
}

// TestResolveReportsWhatIsMissing covers the requirement that a missing
// checkout names what to clone or set rather than failing obscurely.
func TestResolveReportsWhatIsMissing(t *testing.T) {
	parent := t.TempDir()
	repository := filepath.Join(parent, "md-to-tex")

	cases := []struct {
		name    string
		prepare func() string
		want    string
	}{
		{
			name:    "no checkout at all",
			prepare: func() string { return "AGENT_CORE_ROOT" },
			want:    "agent-core checkout not found",
		},
		{
			name: "a checkout carrying no critic profile",
			prepare: func() string {
				checkout(t, parent, false)
				return "AGENT_CORE_ROOT"
			},
			want: "specification-critic profile not found",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.prepare()
			_, err := Resolve(repository, noEnv, os.Stat)
			if err == nil {
				t.Fatal("Resolve() found an install where none is usable")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %q, want it to name %q", err, testCase.want)
			}
		})
	}
}

// TestEnsureBinaryBuildsWhenAbsent covers the rule that a checkout carrying no
// compiled agent is built rather than reported.
func TestEnsureBinaryBuildsWhenAbsent(t *testing.T) {
	install := Install{CoreRoot: t.TempDir()}
	built := ""
	build := func(coreRoot, binary string) error {
		built = binary
		return nil
	}

	binary, err := EnsureBinary(install, os.Stat, build)
	if err != nil {
		t.Fatalf("EnsureBinary() error: %v", err)
	}
	if binary != filepath.Join(install.CoreRoot, "bin", "agent") {
		t.Errorf("binary = %q", binary)
	}
	if built != binary {
		t.Errorf("build target = %q, want %q", built, binary)
	}
}

// TestEnsureBinaryKeepsAnExistingOne covers the other half: a compiled binary
// is used as it stands.
func TestEnsureBinaryKeepsAnExistingOne(t *testing.T) {
	install := Install{CoreRoot: t.TempDir()}
	binDir := filepath.Join(install.CoreRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "agent"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	build := func(string, string) error {
		t.Error("EnsureBinary rebuilt a binary that already existed")
		return nil
	}
	if _, err := EnsureBinary(install, os.Stat, build); err != nil {
		t.Fatalf("EnsureBinary() error: %v", err)
	}
}

// TestEnsureBinaryReportsABuildFailure covers the path where the checkout is
// present but does not compile.
func TestEnsureBinaryReportsABuildFailure(t *testing.T) {
	build := func(string, string) error { return errors.New("compile error") }

	_, err := EnsureBinary(Install{CoreRoot: t.TempDir()}, os.Stat, build)
	if err == nil || !strings.Contains(err.Error(), "build agent binary") {
		t.Errorf("error = %v, want it to name the build", err)
	}
}

// TestCompletedSeparatesAResultFromABrokenInvocation covers the exit-code
// rule: the machine-failed code means the corpus did not pass, and any other
// failure means the run itself broke.
func TestCompletedSeparatesAResultFromABrokenInvocation(t *testing.T) {
	if !Completed(nil) {
		t.Error("a clean exit should count as completed")
	}
	if !Completed(exitError(t, exitMachineFailed)) {
		t.Error("the machine-failed exit code should count as completed")
	}
	if Completed(exitError(t, 1)) {
		t.Error("a general failure should not count as completed")
	}
	if Completed(errors.New("binary not found")) {
		t.Error("a non-exit error should not count as completed")
	}
}

// exitError produces a real *exec.ExitError carrying the given code.
func exitError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+string(rune('0'+code))).Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != code {
		t.Fatalf("could not produce an exit error with code %d: %v", code, err)
	}
	return err
}

// TestFailedReadsTheTerminalStatus covers the output scan, which is what
// catches a failed machine that exits zero.
func TestFailedReadsTheTerminalStatus(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"a passing run", "validate: 9 SRDs — OK\nterminal state: succeeded\n", false},
		{"a failed terminal state", "validate: 9 SRDs — 3 error(s)\nterminal state: failed\n", true},
		{"a failed run line", "run complete: status=failed iterations=1\n", true},
		{"no terminal line at all", "some other output\n", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Failed(testCase.output); got != testCase.want {
				t.Errorf("Failed() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestProfilePathsNameTheTwoRuns covers the two profiles the audit drives: the
// corpus validator and the test-evidence audit.
func TestProfilePathsNameTheTwoRuns(t *testing.T) {
	install := Install{CatalogRoot: filepath.Join("somewhere", "applications", "catalog")}

	if !strings.HasSuffix(install.Profile(), filepath.Join("specification-critic", "profile.yaml")) {
		t.Errorf("Profile() = %q", install.Profile())
	}
	if !strings.HasSuffix(install.AuditProfile(), filepath.Join("specification-critic", "audit-profile.yaml")) {
		t.Errorf("AuditProfile() = %q", install.AuditProfile())
	}
}
