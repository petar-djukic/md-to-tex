// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

// Package service is the HTTP surface over the library (srd010-service):
// parse, convert, and render, each taking the document content in the
// request body and reading no file of its own, plus a readiness probe. The
// same handler set serves a developer's loopback run and the container the
// mesh reaches through a declared REST client.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mdtotex "github.com/petar-djukic/md-to-tex"
	"github.com/petar-djukic/md-to-tex/internal/parse"
	"github.com/petar-djukic/md-to-tex/internal/render"
)

// The routes the service answers.
const (
	PathHealth  = "/healthz"
	PathParse   = "/v1/parse"
	PathConvert = "/v1/convert"
	PathRender  = "/v1/render"
)

// MaxBodyBytes bounds a request body; a manuscript is kilobytes, a corpus
// document at most a few megabytes.
const MaxBodyBytes = 16 << 20

// RenderTimeout bounds one latexmk run.
const RenderTimeout = 3 * time.Minute

// Input is one document as the caller sends it: JSON with a name and the
// markdown source, or the raw markdown as text/markdown with the name in a
// header (srd010-service R1.3).
type Input struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Options are the library's Options as JSON.
type Options struct {
	CitationKeys  []string `json:"citation_keys,omitempty"`
	TableFontSize *string  `json:"table_font_size,omitempty"`
}

func (o Options) library() mdtotex.Options {
	return mdtotex.Options{CitationKeys: o.CitationKeys, TableFontSize: o.TableFontSize}
}

// ConvertRequest converts one chapter, or the title page when kind is
// front-matter.
type ConvertRequest struct {
	Input
	Kind    string  `json:"kind,omitempty"`
	Options Options `json:"options"`
}

// The kinds a chapter converts as.
const (
	KindChapter     = "chapter"
	KindFrontMatter = "front-matter"
)

// ConvertResponse is the library's Result as JSON.
type ConvertResponse struct {
	Name   string          `json:"name"`
	LaTeX  string          `json:"latex"`
	Labels []mdtotex.Label `json:"labels"`
}

// Chapter is one roster entry for a render.
type Chapter struct {
	Input
	Kind string `json:"kind,omitempty"`
}

// RenderRequest is a whole manuscript: the roster in order, the options, the
// container options, and the two files the container names by reference,
// which the caller sends as text because the service reads no file.
type RenderRequest struct {
	Chapters     []Chapter                `json:"chapters"`
	Options      Options                  `json:"options"`
	Container    mdtotex.ContainerOptions `json:"container"`
	Preamble     string                   `json:"preamble"`
	Bibliography string                   `json:"bibliography"`
}

// RenderFailure is what a render that did not produce a PDF returns.
type RenderFailure struct {
	Error      string `json:"error"`
	Stage      string `json:"stage"`
	ExitStatus int    `json:"exit_status,omitempty"`
	Log        string `json:"log,omitempty"`
}

// Problem is an error the library positioned, as JSON.
type Problem struct {
	Error     string `json:"error"`
	Name      string `json:"name,omitempty"`
	Line      int    `json:"line,omitempty"`
	Construct string `json:"construct,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Handler serves the routes.
type Handler struct {
	// Latexmk is the compiler the render route runs; empty means look it
	// up on the path at each render.
	Latexmk string
	// Now is the clock, for tests.
	Now func() time.Time
}

// New returns the service mux.
func New(handler Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+PathHealth, handler.health)
	mux.HandleFunc("POST "+PathParse, handler.parse)
	mux.HandleFunc("POST "+PathConvert, handler.convert)
	mux.HandleFunc("POST "+PathRender, handler.render)
	return mux
}

func (h Handler) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"ok","latexmk":%t}`+"\n", h.latexmk() != "")
}

// latexmk is the compiler's path, or empty when there is none: a configured
// path that does not exist counts as none, so the render route says the
// compiler is absent rather than failing a compile that never started.
func (h Handler) latexmk() string {
	if h.Latexmk != "" {
		if _, err := os.Stat(h.Latexmk); err != nil {
			return ""
		}
		return h.Latexmk
	}
	path, err := exec.LookPath("latexmk")
	if err != nil {
		return ""
	}
	return path
}

func (h Handler) parse(w http.ResponseWriter, r *http.Request) {
	input, err := readInput(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, Problem{Error: "bad_request", Detail: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, parse.Parse([]byte(input.Source), input.Name))
}

func (h Handler) convert(w http.ResponseWriter, r *http.Request) {
	var request ConvertRequest
	if err := readJSON(r, &request); err != nil {
		writeProblem(w, http.StatusBadRequest, Problem{Error: "bad_request", Detail: err.Error()})
		return
	}
	if request.Name == "" {
		request.Name = "document.md"
	}
	result, err := convertOne(request.Input, request.Kind, request.Options.library())
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, problemFor(err))
		return
	}
	writeJSON(w, http.StatusOK, ConvertResponse{Name: result.Name, LaTeX: string(result.LaTeX), Labels: nonNil(result.Labels)})
}

func convertOne(input Input, kind string, options mdtotex.Options) (mdtotex.Result, error) {
	switch kind {
	case "", KindChapter:
		return mdtotex.Convert([]byte(input.Source), input.Name, options)
	case KindFrontMatter:
		return mdtotex.RenderFrontMatter([]byte(input.Source), input.Name, options)
	}
	return mdtotex.Result{}, fmt.Errorf("kind %q is not chapter or front-matter", kind)
}

func (h Handler) render(w http.ResponseWriter, r *http.Request) {
	var request RenderRequest
	if err := readJSON(r, &request); err != nil {
		writeProblem(w, http.StatusBadRequest, Problem{Error: "bad_request", Detail: err.Error()})
		return
	}
	latexmk := h.latexmk()
	if latexmk == "" {
		writeJSON(w, http.StatusServiceUnavailable, RenderFailure{Error: "latexmk_absent", Stage: "compile",
			Log: "latexmk is not on this service's path; the render route needs the TeX Live image"})
		return
	}
	if len(request.Chapters) == 0 {
		writeProblem(w, http.StatusUnprocessableEntity, Problem{Error: "empty_roster", Detail: "a render needs at least one chapter"})
		return
	}
	work, err := os.MkdirTemp("", "md-to-tex-render-*")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, Problem{Error: "workdir", Detail: err.Error()})
		return
	}
	defer os.RemoveAll(work)

	roster := make([]string, 0, len(request.Chapters))
	var results []mdtotex.Result
	for _, chapter := range request.Chapters {
		if chapter.Name == "" {
			writeProblem(w, http.StatusUnprocessableEntity, Problem{Error: "unnamed_chapter", Detail: "every chapter needs a name; it becomes the fragment's file name"})
			return
		}
		result, err := convertOne(chapter.Input, chapter.Kind, request.Options.library())
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, problemFor(err))
			return
		}
		fragment := strings.TrimSuffix(filepath.Base(chapter.Name), filepath.Ext(chapter.Name)) + ".tex"
		if err := os.WriteFile(filepath.Join(work, fragment), result.LaTeX, 0o644); err != nil {
			writeProblem(w, http.StatusInternalServerError, Problem{Error: "write_fragment", Detail: err.Error()})
			return
		}
		roster = append(roster, chapter.Name)
		if chapter.Kind != KindFrontMatter {
			results = append(results, result)
		}
	}
	if collisions := mdtotex.Collisions(results...); len(collisions) > 0 {
		first := collisions[0]
		writeProblem(w, http.StatusUnprocessableEntity, Problem{Error: "label_collision", Construct: first.Identifier,
			Detail: fmt.Sprintf("identifier %q is claimed by more than one chapter", first.Identifier)})
		return
	}
	container, err := mdtotex.GenerateContainer(roster, request.Container)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, Problem{Error: "container", Detail: err.Error()})
		return
	}
	preambleName, bibliographyName := containerNames(request.Container)
	files := map[string]string{
		"main.tex":                string(container),
		preambleName + ".tex":     request.Preamble,
		bibliographyName + ".bib": request.Bibliography,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o644); err != nil {
			writeProblem(w, http.StatusInternalServerError, Problem{Error: "write_" + name, Detail: err.Error()})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), RenderTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, latexmk, "-xelatex", "-interaction=nonstopmode", "-halt-on-error", "-file-line-error", "main.tex")
	command.Dir = work
	// No network and no home: the image expects neither at run time.
	command.Env = append(os.Environ(), "HOME="+work)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	runErr := command.Run()
	pdf, readErr := os.ReadFile(filepath.Join(work, "main.pdf"))
	if runErr != nil || readErr != nil {
		failure := RenderFailure{Error: "compile_failed", Stage: "compile", Log: tail(output.String(), 16000)}
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			failure.ExitStatus = exit.ExitCode()
		}
		if ctx.Err() != nil {
			failure.Error = "compile_timeout"
		}
		writeJSON(w, http.StatusUnprocessableEntity, failure)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprint(len(pdf)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

// containerNames are the preamble and bibliography file stems the container
// names, with the generator's defaults when the caller left them unset.
func containerNames(options mdtotex.ContainerOptions) (string, string) {
	preamble, bibliography := options.Preamble, options.Bibliography
	if preamble == "" {
		preamble = "preamble"
	}
	if bibliography == "" {
		bibliography = "references"
	}
	return preamble, bibliography
}

func problemFor(err error) Problem {
	var positioned *render.Error
	if errors.As(err, &positioned) {
		return Problem{Error: "unmapped_construct", Name: positioned.Name, Line: positioned.Line, Construct: positioned.Construct, Detail: positioned.Detail}
	}
	return Problem{Error: "conversion_failed", Detail: err.Error()}
}

// readInput accepts the JSON envelope or raw markdown (srd010-service R1.3).
func readInput(r *http.Request) (Input, error) {
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, MaxBodyBytes))
	if err != nil {
		return Input{}, err
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		var input Input
		if err := json.Unmarshal(body, &input); err != nil {
			return Input{}, fmt.Errorf("decode JSON body: %w", err)
		}
		if input.Name == "" {
			input.Name = "document.md"
		}
		return input, nil
	}
	name := r.Header.Get("X-Document-Name")
	if name == "" {
		name = "document.md"
	}
	return Input{Name: name, Source: string(body)}, nil
}

func readJSON(r *http.Request, out any) error {
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, MaxBodyBytes))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode JSON body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, problem Problem) { writeJSON(w, status, problem) }

func nonNil(labels []mdtotex.Label) []mdtotex.Label {
	if labels == nil {
		return []mdtotex.Label{}
	}
	return labels
}

func tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[len(text)-limit:]
}
