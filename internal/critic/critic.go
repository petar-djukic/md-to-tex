// Package critic runs the specification-critic over this repository's corpus.
//
// The critic is a deterministic tool pipeline rather than a model: it loads the
// specification corpus, builds a graph of the references between documents, and
// checks that graph for dangling touchpoints, requirement items no acceptance
// criterion covers, test claims naming Go tests that do not exist, and the rest
// of its check set. One agent validates the same corpus format across the
// sibling repositories, which is why this repository stopped hand-building
// those checks.
//
// The agent lives in petar-djukic/declarative-agents, which this repository
// does not vendor. Resolution finds a checkout, builds the binary when the
// checkout carries none, and points the run at this repository's root.
package critic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// exitMachineFailed is the agent's exit code for a run whose machine reached a
// failed terminal state. The process ran correctly and the corpus did not pass,
// which is an audit failure rather than a broken invocation.
const exitMachineFailed = 2

// StatFunc reports whether a path exists, injected so resolution is testable
// without a checkout on disk.
type StatFunc func(string) (os.FileInfo, error)

// Builder compiles the agent binary in a checkout, injected for the same
// reason.
type Builder func(coreRoot, binary string) error

// Install names the two roots a run needs: the agent-core checkout that owns
// the binary and the builtin tool declarations, and the catalog checkout that
// owns the critic's profiles.
type Install struct {
	CoreRoot    string
	CatalogRoot string
}

// Profile is the corpus validator, which checks the graph the specifications
// form.
func (i Install) Profile() string {
	return filepath.Join(i.CatalogRoot, "agents", "specification-critic", "profile.yaml")
}

// AuditProfile is the test-evidence audit, which resolves the test claims in
// the suites against the Go test inventory.
func (i Install) AuditProfile() string {
	return filepath.Join(i.CatalogRoot, "agents", "specification-critic", "audit-profile.yaml")
}

// Resolve finds a declarative-agents checkout for the repository at root.
//
// Explicit environment variables win, so a developer whose checkout sits
// elsewhere is never forced into a directory layout; otherwise we look beside
// this repository, which is where the sibling repositories sit by convention.
func Resolve(root string, getenv func(string) string, stat StatFunc) (Install, error) {
	sibling := filepath.Join(filepath.Dir(root), "declarative-agents")

	coreRoot := firstExistingDir(stat,
		getenv("AGENT_CORE_ROOT"),
		filepath.Join(getenv("DECLARATIVE_AGENTS_ROOT"), "agent-core"),
		filepath.Join(sibling, "agent-core"),
	)
	if coreRoot == "" {
		return Install{}, fmt.Errorf(
			"agent-core checkout not found: clone declarative-agents beside %s, or set AGENT_CORE_ROOT",
			root)
	}

	catalogRoot := firstExistingDir(stat,
		getenv("AGENT_PROFILES_ROOT"),
		filepath.Join(getenv("DECLARATIVE_AGENTS_ROOT"), "applications", "catalog"),
		filepath.Join(sibling, "applications", "catalog"),
		filepath.Join(filepath.Dir(coreRoot), "applications", "catalog"),
	)
	if catalogRoot == "" {
		return Install{}, fmt.Errorf(
			"agent catalog not found: clone declarative-agents beside %s, or set AGENT_PROFILES_ROOT",
			root)
	}

	install := Install{CoreRoot: coreRoot, CatalogRoot: catalogRoot}
	if _, err := stat(install.Profile()); err != nil {
		return Install{}, fmt.Errorf("specification-critic profile not found at %s", install.Profile())
	}
	return install, nil
}

func firstExistingDir(stat StatFunc, candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

// EnsureBinary returns the path to the agent binary, building it when the
// checkout carries none.
func EnsureBinary(install Install, stat StatFunc, build Builder) (string, error) {
	binary := filepath.Join(install.CoreRoot, "bin", "agent")
	if _, err := stat(binary); err == nil {
		return binary, nil
	}
	fmt.Printf("building the agent binary in %s\n", install.CoreRoot)
	if err := build(install.CoreRoot, binary); err != nil {
		return "", fmt.Errorf("build agent binary: %w", err)
	}
	return binary, nil
}

// BuildBinary compiles cmd/agent inside an agent-core checkout.
func BuildBinary(coreRoot, binary string) error {
	command := exec.Command("go", "build", "-o", binary, "./cmd/agent")
	command.Dir = coreRoot
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// Run executes one agent run over target with the given profile, returning
// everything it wrote along with the process error.
//
// The core root is passed explicitly because the shipped profile names its
// tool declarations under the container install path, which the agent remaps
// onto the checkout.
func Run(binary, profile string, install Install, target string) (string, error) {
	command := exec.Command(binary,
		"--profile", profile,
		"--directory", target,
		"--core-root", install.CoreRoot,
	)
	var output bytes.Buffer
	command.Stdout = io.MultiWriter(os.Stdout, &output)
	command.Stderr = io.MultiWriter(os.Stderr, &output)
	err := command.Run()
	return output.String(), err
}

// Completed reports whether the agent process itself ran. A machine that
// reaches a failed terminal state exits non-zero, and that is a result rather
// than a broken invocation.
func Completed(runErr error) bool {
	if runErr == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode() == exitMachineFailed
	}
	return false
}

// Failed reads the terminal status out of a run's output.
//
// The agent reports a failed machine on stdout as well as through its exit
// code, and both are read: a passing exit code with a failed terminal line is
// the shape a silent audit failure takes.
func Failed(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "terminal state: failed") ||
			strings.Contains(line, "run complete: status=failed") {
			return true
		}
	}
	return false
}

// Check runs one profile over target and turns the result into an error.
func Check(root, profile, label string) error {
	install, err := Resolve(root, os.Getenv, os.Stat)
	if err != nil {
		return err
	}
	binary, err := EnsureBinary(install, os.Stat, BuildBinary)
	if err != nil {
		return err
	}

	path := install.Profile()
	if profile == "audit" {
		path = install.AuditProfile()
	}
	fmt.Printf("%s: auditing %s with %s\n", label, root, path)

	output, runErr := Run(binary, path, install, root)
	if !Completed(runErr) {
		return fmt.Errorf("run the specification-critic: %w", runErr)
	}
	if Failed(output) {
		return fmt.Errorf("%s: the specification-critic reported a failed terminal status", label)
	}
	return nil
}
