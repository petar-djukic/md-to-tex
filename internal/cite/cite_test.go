package cite

import (
	"errors"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// citations parses source and returns the citation nodes in document order,
// which is what the renderer core dispatches on.
func citations(t *testing.T, source string) []*Node {
	t.Helper()
	markdown := goldmark.New(goldmark.WithExtensions(Extension))
	document := markdown.Parser().Parse(text.NewReader([]byte(source)))

	var found []*Node
	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if citation, ok := node.(*Node); ok {
				found = append(found, citation)
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return found
}

func render(t *testing.T, node *Node) string {
	t.Helper()
	var builder strings.Builder
	if err := Render(&builder, node); err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	return builder.String()
}

// TestParseRecognisesTheCitationForms covers srd-6-citations R1.1, R1.2, and
// R1.4: single and grouped runs, the key character set, and duplicates kept in
// order.
func TestParseRecognisesTheCitationForms(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "single key",
			source: "Surveyed in [@coronado-2022-ztn-survey].",
			want:   []string{"coronado-2022-ztn-survey"},
		},
		{
			name:   "two keys",
			source: "Both lines converge [@declarative-agents-2026; @zhang-2025b].",
			want:   []string{"declarative-agents-2026", "zhang-2025b"},
		},
		{
			name:   "many keys as the manuscripts write them",
			source: "[@boateng-2024; @zhou-2025-llm-telecom-survey; @tmforum-ig1253; @tmf921a]",
			want:   []string{"boateng-2024", "zhou-2025-llm-telecom-survey", "tmforum-ig1253", "tmf921a"},
		},
		{
			name:   "key characters: digits, colons, periods, slashes, underscores",
			source: "[@etsi-gs-eni-001; @iso/iec_15288:2023; @rfc8342.v2]",
			want:   []string{"etsi-gs-eni-001", "iso/iec_15288:2023", "rfc8342.v2"},
		},
		{
			name:   "no space after the semicolon",
			source: "[@du-2023;@alam-2024]",
			want:   []string{"du-2023", "alam-2024"},
		},
		{
			name:   "duplicates are kept in order",
			source: "[@du-2023; @alam-2024; @du-2023]",
			want:   []string{"du-2023", "alam-2024", "du-2023"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			found := citations(t, testCase.source)
			if len(found) != 1 {
				t.Fatalf("parsed %d citations, want 1", len(found))
			}
			if strings.Join(found[0].Keys, "|") != strings.Join(testCase.want, "|") {
				t.Errorf("Keys = %v, want %v", found[0].Keys, testCase.want)
			}
		})
	}
}

// TestRenderEmitsTheCiteCommand covers srd-6-citations R2.1 and R2.2 with the
// examples the SRD states, and AC1. R2.2 holds because a key carrying an
// underscore reaches the cite command unescaped, which the last case shows.
func TestRenderEmitsTheCiteCommand(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{
			source: "Zero-touch automation is surveyed in [@coronado-2022-ztn-survey].",
			want:   `\cite{coronado-2022-ztn-survey}`,
		},
		{
			source: "Both lines converge [@declarative-agents-2026; @zhang-2025b].",
			want:   `\cite{declarative-agents-2026,zhang-2025b}`,
		},
		{
			source: "An underscore in a key [@iso_iec_15288].",
			want:   `\cite{iso_iec_15288}`,
		},
	}

	for _, testCase := range cases {
		found := citations(t, testCase.source)
		if len(found) != 1 {
			t.Fatalf("parsed %d citations from %q, want 1", len(found), testCase.source)
		}
		if got := render(t, found[0]); got != testCase.want {
			t.Errorf("Render() = %q, want %q", got, testCase.want)
		}
	}
}

// TestValidateReportsAnUnknownKey covers srd-6-citations R3.1 and R3.4 and
// AC2: the first offending key, with the offset a caller turns into a line.
func TestValidateReportsAnUnknownKey(t *testing.T) {
	found := citations(t, "Converge [@du-2023; @absent-key; @also-absent].")
	if len(found) != 1 {
		t.Fatalf("parsed %d citations, want 1", len(found))
	}

	err := Validate(found[0], NewKeySet("du-2023"))
	var unknown *UnknownKeyError
	if !errors.As(err, &unknown) {
		t.Fatalf("Validate() error = %v, want an UnknownKeyError", err)
	}
	if unknown.Key != "absent-key" {
		t.Errorf("Key = %q, want the first offender %q", unknown.Key, "absent-key")
	}
	if unknown.Offset <= 0 {
		t.Errorf("Offset = %d, want the position of the run", unknown.Offset)
	}
}

// TestValidateDistinguishesAbsentFromEmpty covers srd-6-citations R3.2, R3.3,
// and AC3: no key set means validation is off, an empty set means nothing is
// valid, and both sets come from the caller because the library never reads a
// reference corpus.
func TestValidateDistinguishesAbsentFromEmpty(t *testing.T) {
	found := citations(t, "Converge [@du-2023].")
	if len(found) != 1 {
		t.Fatalf("parsed %d citations, want 1", len(found))
	}

	if err := Validate(found[0], nil); err != nil {
		t.Errorf("Validate() with no key set = %v, want nil", err)
	}
	if err := Validate(found[0], KeySet{}); err == nil {
		t.Error("Validate() with an empty key set = nil, want an error")
	}
}

// TestParseDeclinesWhatIsNotACitation covers srd-6-citations R4.1, R4.2, R4.3,
// and R4.5, and AC4.
func TestParseDeclinesWhatIsNotACitation(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"markdown link", "See [the survey](https://example.com/survey)."},
		{"reference link", "See [the survey][survey]."},
		{"image", "![A diagram](fig/diagram.pdf)"},
		{"plain bracketed text", "The array is [a, b, c] in source order."},
		{"unterminated run", "An unterminated [@du-2023 run."},
		{"at sign with no key", "An empty [@] run."},
		{"inline code", "The syntax is `[@du-2023]` in prose."},
		{"escaped bracket", `An escaped \[@du-2023] run.`},
		{"at sign outside brackets", "Mail petar@example.com about it."},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if found := citations(t, testCase.source); len(found) != 0 {
				t.Errorf("parsed %d citations from %q, want none: %v",
					len(found), testCase.source, found[0].Keys)
			}
		})
	}
}

// TestParseGivesBackSentencePunctuation covers srd-6-citations R1.2 and AC6: a
// trailing period or colon belongs to the sentence, not the key.
func TestParseGivesBackSentencePunctuation(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"Surveyed in [@du-2023.]", "du-2023"},
		{"Surveyed in [@du-2023:]", "du-2023"},
		{"Surveyed in [@rfc8342.v2]", "rfc8342.v2"},
	}

	for _, testCase := range cases {
		found := citations(t, testCase.source)
		if len(found) != 1 {
			t.Fatalf("parsed %d citations from %q, want 1", len(found), testCase.source)
		}
		if found[0].Keys[0] != testCase.want {
			t.Errorf("key from %q = %q, want %q", testCase.source, found[0].Keys[0], testCase.want)
		}
	}
}

// TestCitationsParseWhereverInlineContentGoes covers srd-6-citations R2.3: a
// citation is an inline node, so it parses in a heading and a list item as it
// does in a paragraph.
func TestCitationsParseWhereverInlineContentGoes(t *testing.T) {
	const source = `# A heading citing [@du-2023]

- A list item citing [@alam-2024]

> A quotation citing [@zhang-2025b]
`
	found := citations(t, source)
	if len(found) != 3 {
		t.Fatalf("parsed %d citations, want 3", len(found))
	}
	want := []string{"du-2023", "alam-2024", "zhang-2025b"}
	for i, node := range found {
		if node.Keys[0] != want[i] {
			t.Errorf("citation %d = %q, want %q", i, node.Keys[0], want[i])
		}
	}
}

// TestParserIsTriggeredOnTheOpeningBracket covers srd-6-citations R1.3: the
// parser is registered as a goldmark inline parser on the opening bracket, and
// the node it produces holds the keys in order.
func TestParserIsTriggeredOnTheOpeningBracket(t *testing.T) {
	trigger := (&citationParser{}).Trigger()
	if len(trigger) != 1 || trigger[0] != '[' {
		t.Errorf("Trigger() = %q, want [", trigger)
	}

	found := citations(t, "Converge [@du-2023; @alam-2024].")
	if len(found) != 1 {
		t.Fatalf("the extension parsed %d citations, want 1", len(found))
	}
	if found[0].Kind() != Kind {
		t.Errorf("Kind() = %v, want %v", found[0].Kind(), Kind)
	}
}
