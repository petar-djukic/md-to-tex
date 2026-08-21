package render

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
)

// tableFontSize is the size command a table float carries by default: one step
// below the body, which is what the dense tables in the corpus need to fit a
// two-column measure (srd005-tables R2.7).
const tableFontSize = `\footnotesize`

// columnLineWidth is how many characters one column of the two-column layout
// fits on a line at the table font size. It is the divisor in the height
// estimate (srd005-tables R4.2).
const columnLineWidth = 55

// table renders a pipe table and its caption line as a tabularx float
// (srd005-tables R2.1).
func (w *walker) table(node *east.Table, caption tableCaption) error {
	offset := w.offsetOf(node)

	rows, err := w.tableRows(node)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return w.fail(offset, "table", "holds no rows")
	}
	header, body := rows[0], rows[1:]
	columns := len(header)

	for _, row := range rows {
		if len(row) > columns {
			return w.fail(offset, "table",
				fmt.Sprintf("has a row of %d cells where the header states %d; a pipe table cannot express a merged cell",
					len(row), columns))
		}
	}

	// The class states what the measurement cannot see, so it decides on its
	// own; without it the table is measured exactly as before
	// (srd005-tables R4.6, R4.7).
	wide := caption.wide || spansBothColumns(columns, rows, body)
	environment, measure := "table", `\columnwidth`
	if wide {
		environment, measure = "table*", `\textwidth`
	}

	w.out.WriteString(`\begin{` + environment + "}[!t]\n")
	w.out.WriteString(`\caption{` + caption.text + "}\n")
	w.out.WriteString(`\label{` + caption.identifier + "}\n")
	w.out.WriteString("\\centering\n")
	if size := w.config.TableFontSize(); size != "" {
		w.out.WriteString(size + "\n")
	}
	w.out.WriteString(`\begin{tabularx}{` + measure + `}{` + columnSpec(columns, body, [][]string{header}) + "}\n")
	w.out.WriteString("\\toprule\n")
	w.out.WriteString(strings.Join(header, " & ") + ` \\` + "\n")
	w.out.WriteString("\\midrule\n")
	for _, row := range body {
		w.out.WriteString(strings.Join(pad(row, columns), " & ") + ` \\` + "\n")
	}
	w.out.WriteString("\\bottomrule\n")
	w.out.WriteString("\\end{tabularx}\n")
	w.out.WriteString(`\end{` + environment + "}\n\n")
	return nil
}

// pad fills a short row so every row states the same number of cells.
func pad(row []string, columns int) []string {
	for len(row) < columns {
		row = append(row, "")
	}
	return row
}

// tableRows renders every cell through the inline path, so escaping, emphasis,
// and citations behave in a cell as they do in a paragraph
// (srd005-tables R2.6).
func (w *walker) tableRows(node *east.Table) ([][]string, error) {
	var rows [][]string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *east.TableHeader:
			row, err := w.tableRow(typed)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		case *east.TableRow:
			row, err := w.tableRow(typed)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (w *walker) tableRow(node ast.Node) ([]string, error) {
	var row []string
	for cell := node.FirstChild(); cell != nil; cell = cell.NextSibling() {
		rendered, err := w.inlineString(cell)
		if err != nil {
			return nil, err
		}
		row = append(row, strings.TrimSpace(rendered))
	}
	return row, nil
}

// spansBothColumns decides whether a table takes the two-column float
// (srd005-tables R4.1, R4.2, R4.3).
//
// Column count alone is the wrong test: a two-column table of 200-character
// cells is as cramped as a five-column table of short ones, and a long table
// of modest cells still runs as a tall ribbon. The bars are set high because
// spanning turns a table into a float that leaves its text behind (R4.4).
func spansBothColumns(columns int, rows, body [][]string) bool {
	if columns >= 5 {
		return true
	}

	widest := 0
	for _, row := range rows {
		for _, cell := range row {
			widest = max(widest, len(cell))
		}
	}
	if widest*columns > 350 {
		return true
	}

	estimated := 0
	for _, row := range body {
		widestCell := 0
		for _, cell := range row {
			widestCell = max(widestCell, len(cell))
		}
		lines := int(math.Ceil(float64(widestCell*columns) / columnLineWidth))
		estimated += max(1, lines)
	}
	return estimated > 45
}

// columnSpec computes the tabularx column specification: one weighted X column
// per column, ragged right, with the array backslash restored
// (srd005-tables R3.6).
func columnSpec(columns int, body, header [][]string) string {
	weights := columnWeights(columns, body, header)
	if weights == nil {
		// Nothing measurable, so every column takes the same share
		// (srd005-tables R3.7).
		return strings.TrimSpace(strings.Repeat("X ", columns))
	}

	parts := make([]string, columns)
	for i, weight := range weights {
		parts[i] = fmt.Sprintf(`>{\hsize=%.3f\hsize\raggedright\arraybackslash}X`, weight)
	}
	return strings.Join(parts, " ")
}

// controlSequence matches the LaTeX a rendered cell carries, which is not part
// of the word an author can see (srd005-tables R3.4).
var controlSequence = regexp.MustCompile(`\\[A-Za-z]+`)

// columnWeights divides the measure among the columns by what their cells
// hold (srd005-tables R3.1 through R3.5).
//
// Equal-width columns waste the page: a column of three words takes the same
// space as a column of comma lists, so the long one wraps to five lines while
// the short one runs empty.
func columnWeights(columns int, body, header [][]string) []float64 {
	totals := make([]float64, columns)
	counts := make([]float64, columns)
	longest := make([]float64, columns)

	measure := func(rows [][]string) {
		for _, row := range rows {
			for i, cell := range row {
				if i >= columns {
					continue
				}
				totals[i] += float64(len(cell))
				counts[i]++
				for _, word := range strings.Fields(cell) {
					plain := strings.NewReplacer("{", "", "}", "").Replace(controlSequence.ReplaceAllString(word, ""))
					longest[i] = math.Max(longest[i], float64(len(plain)))
				}
			}
		}
	}

	measure(body)
	if !anyMeasured(counts) {
		// An all-header table still needs a basis (srd005-tables R3.1).
		measure(header)
	}
	if !anyMeasured(counts) {
		// Nothing to weigh at all, so the caller falls back to equal-width
		// columns rather than dividing a measure by guesswork
		// (srd005-tables R3.7).
		return nil
	}

	means := make([]float64, columns)
	sum := 0.0
	for i := range means {
		mean := 1.0
		if counts[i] > 0 {
			mean = totals[i] / counts[i]
		}
		// Square-root damping: ten times the text needs about three times the
		// width, because long text wraps and short text cannot shrink below
		// its longest word (srd005-tables R3.2).
		means[i] = math.Sqrt(math.Max(1, mean))
		sum += means[i]
	}
	if sum <= 0 {
		return nil
	}

	rowChars := 0.0
	for _, word := range longest {
		rowChars += word
	}

	weights := make([]float64, columns)
	for i := range weights {
		weight := means[i] * float64(columns) / sum
		floor := 0.45
		if rowChars > 0 {
			floor = math.Max(floor, longest[i]*float64(columns)/rowChars*0.85)
		}
		weights[i] = math.Min(math.Max(weight, floor), 2.2)
	}

	// Flooring and clamping break the sum, so rescale to restore it
	// (srd005-tables R3.3, R3.5).
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	scale := float64(columns) / total
	for i := range weights {
		weights[i] *= scale
	}
	return weights
}

func anyMeasured(counts []float64) bool {
	for _, count := range counts {
		if count > 0 {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
