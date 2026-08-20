//go:build mage

// Build targets for md-to-tex.
//
//	audit  the specification corpus, the road map, then vet and the tests
//	specs  the corpus checks alone: the critic and the test-evidence audit
//	test   go test ./...
//	lint   go vet ./...
//
// Audit is the gate, and it runs docs before code: a corpus that contradicts
// itself is cheaper to read about than a compile error in code written against
// the specification it contradicts.
//
// The corpus checks are the specification-critic from
// petar-djukic/declarative-agents, which validates the same format across the
// sibling repositories. It is found beside this repository or through
// AGENT_CORE_ROOT and AGENT_PROFILES_ROOT. The road-map check is this
// repository's own, because the shared schema has no field for it, and it runs
// under go test with no checkout present.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"

	"github.com/petar-djukic/md-to-tex/internal/critic"
	"github.com/petar-djukic/md-to-tex/internal/specs"
)

// Default is what mage runs with no target named.
var Default = Audit

// Audit runs every check the repository has, docs before code.
func Audit() error {
	if err := Specs(); err != nil {
		return err
	}
	mg.Deps(Lint)
	return Test()
}

// Specs runs the corpus checks: the specification-critic over the graph the
// documents form, the test-evidence audit over the claims the suites make, and
// the road-map release edge.
func Specs() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := critic.Check(root, "corpus", "corpus"); err != nil {
		return err
	}
	if err := critic.Check(root, "audit", "test evidence"); err != nil {
		return err
	}

	report, err := specs.Check(root)
	if err != nil {
		return err
	}
	fmt.Print(report.Summary())
	return report.Err()
}

// Test runs the Go test suite, which includes the road-map check.
func Test() error {
	return run("go", "test", "./...")
}

// Lint vets the Go code.
func Lint() error {
	return run("go", "vet", "./...")
}

func run(name string, args ...string) error {
	fmt.Println("+", name, args)
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
