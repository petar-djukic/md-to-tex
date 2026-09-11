//go:build mage

// Build targets for md-to-tex.
//
//	audit       the specification corpus, the road map, then vet and the tests
//	specs       the corpus checks alone: the critic and the test-evidence audit
//	test        go test ./...
//	lint        go vet ./...
//	image       build the service image on the TeX Live base
//	imageSmoke  start the image, render the fixture manuscript, require a PDF
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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

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

// imageTag is the tag the image build produces and the smoke runs.
const imageTag = "md-to-tex:dev"

// Image builds the service image on the TeX Live base (srd010-service R5.1).
func Image() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("image: docker is not on the path")
	}
	return run("docker", "build", "-t", imageTag, ".")
}

// ImageSmoke starts the built image, waits on its health route, renders the
// fixture manuscript through the service, and requires a PDF back
// (srd010-service R5.2). It skips, naming why, when Docker or the image is
// absent, so the audit stays runnable on a host without either.
func ImageSmoke() error {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("imageSmoke: SKIP docker is not on the path")
		return nil
	}
	if err := exec.Command("docker", "image", "inspect", imageTag).Run(); err != nil {
		fmt.Printf("imageSmoke: SKIP the image %s is not built; run mage image\n", imageTag)
		return nil
	}
	name := fmt.Sprintf("md-to-tex-smoke-%d", time.Now().UnixNano())
	if err := run("docker", "run", "-d", "--rm", "--name", name, "-p", "127.0.0.1:18090:8090", imageTag); err != nil {
		return err
	}
	defer func() { _ = exec.Command("docker", "stop", name).Run() }()

	base := "http://127.0.0.1:18090"
	deadline := time.Now().Add(60 * time.Second)
	for {
		response, err := http.Get(base + "/healthz")
		if err == nil && response.StatusCode == http.StatusOK {
			body, _ := readAll(response)
			if !strings.Contains(body, `"latexmk":true`) {
				return fmt.Errorf("imageSmoke: the image's health route reports no compiler: %s", body)
			}
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("imageSmoke: %s/healthz never answered 200: %v", base, err)
		}
		time.Sleep(time.Second)
	}

	request, err := smokeRenderRequest()
	if err != nil {
		return err
	}
	response, err := http.Post(base+"/v1/render", "application/json", bytes.NewReader(request))
	if err != nil {
		return err
	}
	body, _ := readAll(response)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(body, "%PDF") {
		if len(body) > 4000 {
			body = body[len(body)-4000:]
		}
		return fmt.Errorf("imageSmoke: render answered %d %s: %s", response.StatusCode, response.Header.Get("Content-Type"), body)
	}
	fmt.Printf("imageSmoke: PASS the image rendered the fixture manuscript to a %d-byte PDF\n", len(body))
	return nil
}

// smokeRenderRequest is the fixture manuscript as the service's render
// request: the three chapters, the keys they cite, and the preamble and
// bibliography the container names.
func smokeRenderRequest() ([]byte, error) {
	chapters := []map[string]string{}
	for _, name := range []string{"00-front-matter.md", "01-introduction.md", "02-floats.md"} {
		source, err := os.ReadFile("testdata/manuscript/" + name)
		if err != nil {
			return nil, err
		}
		chapter := map[string]string{"name": name, "source": string(source)}
		if name == "00-front-matter.md" {
			chapter["kind"] = "front-matter"
		}
		chapters = append(chapters, chapter)
	}
	preamble, err := os.ReadFile("testdata/render/preamble.tex")
	if err != nil {
		return nil, err
	}
	bibliography, err := os.ReadFile("testdata/render/references.bib")
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"chapters":     chapters,
		"options":      map[string]any{"citation_keys": []string{"du-2023", "coronado-2022-ztn-survey"}},
		"preamble":     string(preamble),
		"bibliography": string(bibliography),
	})
}

func readAll(response *http.Response) (string, error) {
	defer response.Body.Close()
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(response.Body)
	return buffer.String(), err
}
