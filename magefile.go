//go:build mage

// Build targets for md-to-tex.
//
//	audit  the documentation layer, then build, vet, and the tests
//	test   go test ./...
//	lint   go vet ./...
//
// Audit is the gate: it dispatches the documentation-layer checks to go test
// so they run identically with or without mage installed, then holds the Go
// code to the same bar.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
)

// Default is what mage runs with no target named.
var Default = Audit

// Audit runs every check the repository has, docs before code. The
// documentation layer comes first because a dangling specification pointer is
// cheaper to read about than a compile error in code written against the
// specification it points at.
func Audit() error {
	if err := Specs(); err != nil {
		return err
	}
	mg.Deps(Lint)
	return Test()
}

// Specs runs the documentation-layer checks and prints their summary: strict
// YAML parsing, the architecture-to-SRD edges in both directions, acceptance
// criteria tracing to requirements that exist, and requirement coverage.
func Specs() error {
	return run("go", "test", "./internal/specs/...", "-run", "^TestDocsLayer$", "-v")
}

// Test runs the Go test suite, which includes the documentation-layer checks.
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
