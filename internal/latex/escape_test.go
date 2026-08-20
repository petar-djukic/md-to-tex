package latex

import "testing"

// TestEscapeReplacesTheTenSpecials covers srd-3-escaping R1.1 and R1.3 with
// the example the SRD states, and AC1: nothing outside the ten changes.
func TestEscapeReplacesTheTenSpecials(t *testing.T) {
	const source = `100% of the R&D budget, item_one, and $x$ in {braces}`
	const want = `100\% of the R\&D budget, item\_one, and \$x\$ in \{braces\}`

	if got := Escape(source); got != want {
		t.Errorf("Escape() = %q, want %q", got, want)
	}
}

// TestEscapeReplacesEachSpecialOnce covers srd-3-escaping R1.1 character by
// character, so a table entry that goes missing names itself.
func TestEscapeReplacesEachSpecialOnce(t *testing.T) {
	cases := []struct {
		special string
		want    string
	}{
		{`\`, `\textbackslash{}`},
		{`&`, `\&`},
		{`%`, `\%`},
		{`$`, `\$`},
		{`#`, `\#`},
		{`_`, `\_`},
		{`{`, `\{`},
		{`}`, `\}`},
		{`~`, `\textasciitilde{}`},
		{`^`, `\textasciicircum{}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.special, func(t *testing.T) {
			if got := Escape(testCase.special); got != testCase.want {
				t.Errorf("Escape(%q) = %q, want %q", testCase.special, got, testCase.want)
			}
		})
	}
}

// TestEscapeIsSinglePass covers srd-3-escaping R1.2 and AC2: the backslashes
// the replacements introduce are not themselves escaped.
func TestEscapeIsSinglePass(t *testing.T) {
	const source = `A backslash \ and a tilde ~ and a caret ^`
	const want = `A backslash \textbackslash{} and a tilde \textasciitilde{} and a caret \textasciicircum{}`

	if got := Escape(source); got != want {
		t.Errorf("Escape() = %q, want %q", got, want)
	}
}

// TestEscapePassesUnicodeThrough covers srd-3-escaping R2.1, R2.2, and R2.3,
// and AC3. R2.2 is the xelatex assumption that makes passing Unicode through
// safe: these strings compile as written under a Unicode-aware engine and are
// what the corpus actually carries.
func TestEscapePassesUnicodeThrough(t *testing.T) {
	cases := []string{
		"Bianchi's model of 802.11 uses λ and μ -- see Figure 1",
		"Djukić, Ångström, Erdős, and Šebesta",
		"α β γ Δ Σ Ω ∈ ∀ ∫ ≤ ≥ ≈",
		"en–dash, em—dash, “curly quotes”, ‘singles’, and an ellipsis…",
	}

	for _, source := range cases {
		if got := Escape(source); got != source {
			t.Errorf("Escape(%q) = %q, want it unchanged", source, got)
		}
	}
}

// TestEscapeMatchesTheReferenceImplementation covers srd-3-escaping R1.1 and AC4: the
// output matches paperkit's escapeLaTeX, which the manuscripts are written
// against. The fixtures are the title and author fields from paperkit's own
// frontmatter tests.
func TestEscapeMatchesTheReferenceImplementation(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "title with no specials",
			source: "A Reference Architecture for L5 Autonomous Networks",
			want:   "A Reference Architecture for L5 Autonomous Networks",
		},
		{
			name:   "subtitle with a colon",
			source: "Level 5 Autonomy: What It Would Take",
			want:   "Level 5 Autonomy: What It Would Take",
		},
		{
			name:   "author with an email address",
			source: "petar.djukic@example.com",
			want:   "petar.djukic@example.com",
		},
		{
			name:   "title carrying an ampersand and an underscore",
			source: "R&D on intent_driven networks",
			want:   `R\&D on intent\_driven networks`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Escape(testCase.source); got != testCase.want {
				t.Errorf("Escape() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestEscapeLeavesOrdinaryPunctuationAlone covers srd-3-escaping R1.3: the at
// sign, brackets, and parentheses are not LaTeX markup and are not escaped.
func TestEscapeLeavesOrdinaryPunctuationAlone(t *testing.T) {
	const source = `[@key] (parenthesised) <angled> "quoted" 'single' a|b +c= -d/e`

	if got := Escape(source); got != source {
		t.Errorf("Escape(%q) = %q, want it unchanged", source, got)
	}
}

// TestEscapeIsAPlainFunction covers srd-3-escaping R3.1: a pure function from
// a string to a string, with no options argument and no error return. The
// assignment fails to compile if the signature grows either.
func TestEscapeIsAPlainFunction(t *testing.T) {
	var escape func(string) string = Escape

	if escape("plain") != "plain" {
		t.Error("Escape does not round-trip a string with no specials")
	}
}
