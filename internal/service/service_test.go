// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manuscript(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manuscript", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func post(t *testing.T, handler http.Handler, path, contentType, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestHealthAnswers(t *testing.T) {
	handler := New(Handler{Latexmk: "/nonexistent/latexmk"})
	request := httptest.NewRequest(http.MethodGet, PathHealth, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("health = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestParseAcceptsJSONAndRawMarkdown(t *testing.T) {
	handler := New(Handler{})
	source := manuscript(t, "01-introduction.md")
	envelope, _ := json.Marshal(map[string]string{"name": "01-introduction.md", "source": source})
	asJSON := post(t, handler, PathParse, "application/json", string(envelope))
	asRaw := post(t, handler, PathParse, "text/markdown", source, "X-Document-Name", "01-introduction.md")
	if asJSON.Code != http.StatusOK || asRaw.Code != http.StatusOK {
		t.Fatalf("parse = %d / %d", asJSON.Code, asRaw.Code)
	}
	if asJSON.Body.String() != asRaw.Body.String() {
		t.Fatal("the JSON envelope and the raw body parse differently")
	}
	var parsed struct {
		Name     string `json:"name"`
		Headings []struct {
			Identifier string `json:"identifier"`
		} `json:"headings"`
		Segments  []json.RawMessage `json:"segments"`
		Citations []json.RawMessage `json:"citations"`
	}
	if err := json.Unmarshal(asJSON.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Name != "01-introduction.md" || len(parsed.Headings) == 0 || len(parsed.Segments) == 0 || len(parsed.Citations) == 0 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if bad := post(t, handler, PathParse, "application/json", "{not json"); bad.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON = %d", bad.Code)
	}
}

func TestConvertRendersAChapterAndPositionsAnError(t *testing.T) {
	handler := New(Handler{})
	request, _ := json.Marshal(ConvertRequest{
		Input:   Input{Name: "01-introduction.md", Source: manuscript(t, "01-introduction.md")},
		Options: Options{CitationKeys: []string{"du-2023", "coronado-2022-ztn-survey"}},
	})
	ok := post(t, handler, PathConvert, "application/json", string(request))
	if ok.Code != http.StatusOK {
		t.Fatalf("convert = %d %s", ok.Code, ok.Body.String())
	}
	var response ConvertResponse
	if err := json.Unmarshal(ok.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("..", "..", "testdata", "expected", "01-introduction.tex"))
	if err != nil {
		t.Fatal(err)
	}
	if response.LaTeX != string(expected) || len(response.Labels) == 0 {
		t.Fatalf("convert differs from the library's committed expectation, or reports no labels")
	}

	front, _ := json.Marshal(ConvertRequest{Input: Input{Name: "00-front-matter.md", Source: manuscript(t, "00-front-matter.md")}, Kind: KindFrontMatter})
	if page := post(t, handler, PathConvert, "application/json", string(front)); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `\\title{`) {
		t.Fatalf("front matter convert = %d %s", page.Code, page.Body.String())
	}

	broken, _ := json.Marshal(ConvertRequest{Input: Input{Name: "broken.md", Source: "# Heading\n\nSome text\n\n---\n\nmore\n"}})
	failed := post(t, handler, PathConvert, "application/json", string(broken))
	if failed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unmapped construct = %d %s", failed.Code, failed.Body.String())
	}
	var problem Problem
	if err := json.Unmarshal(failed.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Error != "unmapped_construct" || problem.Name != "broken.md" || problem.Line != 5 || problem.Construct != "thematic break" {
		t.Fatalf("problem = %+v", problem)
	}
}

func TestRenderWithoutLatexmkSaysSo(t *testing.T) {
	handler := New(Handler{Latexmk: "/nonexistent/latexmk"})
	request, _ := json.Marshal(RenderRequest{Chapters: []Chapter{{Input: Input{Name: "a.md", Source: "# A\n\ntext\n"}}}})
	response := post(t, handler, PathRender, "application/json", string(request))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "latexmk_absent") {
		t.Fatalf("render without latexmk = %d %s, want 503 naming the compiler", response.Code, response.Body.String())
	}
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, PathHealth, nil))
	if !strings.Contains(health.Body.String(), `"latexmk":false`) {
		t.Fatalf("health does not report the absent compiler: %s", health.Body.String())
	}
}

func TestRenderRefusesAnEmptyRosterAndACollision(t *testing.T) {
	// A compiler that exists, so the roster and collision checks run before
	// any compile would; neither case reaches it.
	fake := filepath.Join(t.TempDir(), "latexmk")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	handler := New(Handler{Latexmk: fake})
	empty, _ := json.Marshal(RenderRequest{})
	if response := post(t, handler, PathRender, "application/json", string(empty)); response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "empty_roster") {
		t.Fatalf("empty roster = %d %s", response.Code, response.Body.String())
	}
	collision, _ := json.Marshal(RenderRequest{Chapters: []Chapter{
		{Input: Input{Name: "a.md", Source: "# Intro\n\ntext\n"}},
		{Input: Input{Name: "b.md", Source: "# Intro\n\ntext\n"}},
	}})
	if response := post(t, handler, PathRender, "application/json", string(collision)); response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "label_collision") {
		t.Fatalf("collision = %d %s", response.Code, response.Body.String())
	}
}

// TestRenderRunsTheCompilerAndAnswersThePDF drives the render route with a
// compiler stand-in: a script that checks the working directory holds the
// container, every fragment, the preamble, and the bibliography, and writes
// a PDF. The route's plumbing is proven here on any host; the compile itself
// is proven inside the image by mage imageSmoke, where latexmk is real.
func TestRenderRunsTheCompilerAndAnswersThePDF(t *testing.T) {
	record := filepath.Join(t.TempDir(), "args")
	fake := filepath.Join(t.TempDir(), "latexmk")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + record + "\n" +
		"for f in main.tex 00-front-matter.tex 01-introduction.tex 02-floats.tex preamble.tex references.bib; do [ -f \"$f\" ] || { echo missing $f; exit 3; }; done\n" +
		"grep -q 'input{01-introduction}' main.tex || exit 4\n" +
		"printf '%%PDF-1.7 fake\\n' > main.pdf\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	response := renderManuscript(t, New(Handler{Latexmk: fake}))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(response.Body.String(), "%PDF") {
		t.Fatalf("render = %d %s: %s", response.Code, response.Header().Get("Content-Type"), tail(response.Body.String(), 2000))
	}
	args, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "-xelatex") || !strings.Contains(string(args), "main.tex") {
		t.Fatalf("latexmk was invoked with %q, want xelatex over main.tex", args)
	}

	failing := filepath.Join(t.TempDir(), "latexmk")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\necho 'LaTeX Error: something' >&2\nexit 12\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	failed := renderManuscript(t, New(Handler{Latexmk: failing}))
	var failure RenderFailure
	if err := json.Unmarshal(failed.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode failure: %v: %s", err, failed.Body.String())
	}
	if failed.Code != http.StatusUnprocessableEntity || failure.Error != "compile_failed" || failure.ExitStatus != 12 || !strings.Contains(failure.Log, "LaTeX Error") {
		t.Fatalf("failed render = %d %+v", failed.Code, failure)
	}
}

func renderManuscript(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	request, _ := json.Marshal(ManuscriptRenderRequest(t))
	return post(t, handler, PathRender, "application/json", string(request))
}

// ManuscriptRenderRequest is the fixture manuscript as a render request: the
// three chapters, the citation keys, and the preamble and bibliography under
// testdata/render.
func ManuscriptRenderRequest(t *testing.T) RenderRequest {
	t.Helper()
	return RenderRequest{
		Chapters: []Chapter{
			{Input: Input{Name: "00-front-matter.md", Source: manuscript(t, "00-front-matter.md")}, Kind: KindFrontMatter},
			{Input: Input{Name: "01-introduction.md", Source: manuscript(t, "01-introduction.md")}},
			{Input: Input{Name: "02-floats.md", Source: manuscript(t, "02-floats.md")}},
		},
		Options:      Options{CitationKeys: []string{"du-2023", "coronado-2022-ztn-survey"}},
		Preamble:     renderFixture(t, "preamble.tex"),
		Bibliography: renderFixture(t, "references.bib"),
	}
}

// renderFixture reads one of the two files the container names by reference,
// kept under testdata/render so the image smoke sends the same ones.
func renderFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "render", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
