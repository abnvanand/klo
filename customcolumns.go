// Copyright 2019 Harald Albrecht.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package klo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"text/tabwriter"

	"k8s.io/client-go/util/jsonpath"
)

// CustomColumnsPrinter prints neatly formatted tables with custom columns.
type CustomColumnsPrinter struct {
	// The individual columns with their headers and JSONPath expressions.
	Columns []*Column
	// Hide column headers
	HideHeaders bool
	// Padding between columns
	Padding int
	// NoWrapSlices disables the expansion of slice-valued cells into one
	// element per line; slices are then printed as "[a b c]" on a single line.
	NoWrapSlices bool
}

// Column stores the header text and the JSONPath for fetching column values.
// In addition, it features a column name, which is used to identify a
// specific column when reporting errors.
type Column struct {
	Name     string             // Column name, for error reporting.
	Header   string             // Column header text.
	Template *jsonpath.JSONPath // Compiled JSONPath expression.
	Raw      string             // Original JSONPath expression.
}

// NewCustomColumnsPrinterFromSpec returns a new custom columns printer for the
// given specification. This specification is in form of a string consisting of
// a series of <column-header-name>:<json-path-expr> elements, separated by ",".
// The default padding between columns is set to 0, but can be changed later
// using the Padding field of the printer returned.
func NewCustomColumnsPrinterFromSpec(spec string) (ValuePrinter, error) {
	if spec == "" {
		return nil, errors.New("no custom columns given")
	}
	// Split the columns specification into individual columns, and then get
	// the column header text as well as the JSONPath expression for each
	// column.
	ccp := &CustomColumnsPrinter{
		Padding: 1,
	}
	templcols := strings.Split(spec, ",")
	columns := make([]*Column, len(templcols))
	for idx, part := range templcols {
		columnspec := strings.SplitN(part, ":", 2)
		if len(columnspec) != 2 {
			return nil, fmt.Errorf("unexpected custom-columns spec: %s, expected <header>:<json-path-expr>", part)
		}
		cc := &Column{
			Name:   fmt.Sprintf("column%d", idx+1),
			Header: columnspec[0],
		}
		if err := cc.SetExpression(columnspec[1]); err != nil {
			return nil, err
		}
		columns[idx] = cc
	}
	ccp.Columns = columns
	return ccp, nil
}

// NewCustomColumnsPrinterFromTemplate returns a new custom columns printer
// for a template read from the given template stream. The template must
// consist of two lines, the first specifying the column headers, and the
// second giving the JSONPath expressions for each column. The
func NewCustomColumnsPrinterFromTemplate(tr io.Reader) (ValuePrinter, error) {
	const expectedformat = "expected format is one line of space-separated column headers, and one line of space-separated JSONPath expressions"
	sc := bufio.NewScanner(tr)
	if !sc.Scan() {
		return nil, fmt.Errorf("template is missing the header line; %s", expectedformat)
	}
	columnheaders := strings.Fields(sc.Text())
	if !sc.Scan() {
		return nil, fmt.Errorf("template is missing the JSON expressions line; %s", expectedformat)
	}
	columnexprs := strings.Fields(sc.Text())
	if len(columnheaders) != len(columnexprs) {
		return nil, fmt.Errorf("number of column headers (%d) does not match number of JSON expressions (%d)",
			len(columnheaders), len(columnexprs))
	}
	if len(columnheaders) == 0 {
		return nil, fmt.Errorf("no columns specified; %s", expectedformat)
	}
	ccp := &CustomColumnsPrinter{
		Padding: 1,
	}
	columns := make([]*Column, len(columnheaders))
	for idx := range columnheaders {
		cc := &Column{
			Name:   fmt.Sprintf("column%d", idx+1),
			Header: columnheaders[idx],
		}
		if err := cc.SetExpression(columnexprs[idx]); err != nil {
			return nil, err
		}
		columns[idx] = cc
	}
	ccp.Columns = columns
	return ccp, nil
}

// Fprint prints the value v in a neatly formatted table according to the
// custom-column spec or template given when creating this custom-columns
// printer. The table is then written to the specified writer. If this writer
// is already a tabwriter, then it is the caller's responsibility to flush the
// tabwriter when it's the right point to do so.
func (p *CustomColumnsPrinter) Fprint(w io.Writer, v interface{}) error {
	// If the writer given isn't a tabwriter, let's wrap it into one! And only
	// then ensure that the tabbed table gets flushed, so the column widths
	// get calculated and the columns properly aligned. If the caller gave us
	// a tabwriter, then it is her/his responsibility to flush the tabwriter
	// table when necessary.
	if _, ok := w.(*tabwriter.Writer); !ok {
		tw := tabwriter.NewWriter(w, 5, 8, p.Padding, ' ', 0)
		defer tw.Flush()
		w = tw
	}
	// Print column headers ... but only if not hidden...
	if !p.HideHeaders {
		headers := make([]string, len(p.Columns))
		for idx, column := range p.Columns {
			headers[idx] = column.Header
		}
		fmt.Fprintln(w, strings.Join(headers, "\t"))
	}
	// Print value(s)...
	if v != nil {
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			sl := reflect.ValueOf(v)
			for idx := 0; idx < sl.Len(); idx++ {
				// Work on a single row and now find the results of all columns
				// for the current object...
				rowval := sl.Index(idx).Interface()
				if rv, ok := rowval.(reflect.Value); ok {
					rowval = rv.Interface()
				}
				if err := p.printrow(w, rowval); err != nil {
					return err
				}
			}
		} else {
			if rv, ok := v.(reflect.Value); ok {
				v = rv.Interface()
			}
			return p.printrow(w, v)
		}
	}
	return nil
}

// printrow prints a single row, that is, a single row object. If a cell
// resolves to a slice, its elements are printed one per line, with the other
// columns left blank on the continuation lines.
func (p *CustomColumnsPrinter) printrow(w io.Writer, rowval interface{}) error {
	cells := make([][]string, len(p.Columns))
	height := 1
	for cidx, col := range p.Columns {
		// Calculate the result of a this column for the current row.
		res, err := col.Template.FindResults(rowval)
		if err != nil {
			return err
		}
		// Depending on the JSONPath expression, the result for this
		// column might consist of multiple values, or even none at
		// all.
		if len(res) == 0 || len(res[0]) == 0 {
			cells[cidx] = []string{"<none>"}
		} else {
			cells[cidx] = cellLines(res, p.NoWrapSlices)
		}
		if len(cells[cidx]) > height {
			height = len(cells[cidx])
		}
	}
	// Print this row by printing all columns' values, separated by god'ol
	// horizontal tab control chars.
	line := make([]string, len(p.Columns))
	for l := 0; l < height; l++ {
		for cidx := range p.Columns {
			line[cidx] = ""
			if l < len(cells[cidx]) {
				line[cidx] = cells[cidx][l]
			}
		}
		if _, err := fmt.Fprintln(w, strings.Join(line, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// cellLines turns a JSONPath expression result into the lines of a cell. A
// result consisting of a single slice (or array) value is expanded into one
// line per element, unless nowrap is set; anything else is stringified into
// a single line.
func cellLines(res [][]reflect.Value, nowrap bool) []string {
	if !nowrap && len(res) == 1 && len(res[0]) == 1 {
		v := res[0][0]
		for v.IsValid() && v.Kind() == reflect.Interface {
			v = v.Elem()
		}
		if v.IsValid() && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) &&
			v.Type().Elem().Kind() != reflect.Uint8 {
			if v.Len() == 0 {
				return []string{"<none>"}
			}
			lines := make([]string, v.Len())
			for i := range lines {
				lines[i] = fmt.Sprintf("%v", v.Index(i).Interface())
			}
			return lines
		}
	}
	return []string{stringFromJSONExprResult(res, ", ")}
}

// Stringifies a JSONPath expression result.
func stringFromJSONExprResult(res [][]reflect.Value, sep string) string {
	vals := []string{}
	for arridx := range res {
		for validx := range res[arridx] {
			vals = append(
				vals,
				fmt.Sprintf("%v", res[arridx][validx].Interface()))
		}
	}
	return strings.Join(vals, sep)
}

// See: github.com/kubernetes/pkg/kubectl/cmd/get/customcolumn.go; please note
// that this JSONPath regexp just checks that a JSONPath expression is either
// enclosed by curly braces, or not at all. And it checks that there is an
// optional leading dot. And it finally checks that there are no curly braces
// inside JSONPath expression again. Nothing more. Not much, after all.
var jsonPathRegexp = regexp.MustCompile(`^\{\.?([^{}]+)\}$|^\.?([^{}]+)$`)

// SetExpression sets the JSONPath expression for a specific column. It
// accepts a more relaxed JSONPath expression syntax in the same way kubectl
// does for its custom columns. In particular, it accepts:
//   * x.y.z ... without leading "." or curly braces.
//   * {x.y.z} ... without leading ".", but at least curly braces.
//   * .x.y.z ... without curly braces.
//   * {.x.y.z} ... and finally as "standard".
// Additionally, the empty expression "" also gets accepted.
func (c *Column) SetExpression(exp string) error {
	if exp == "" {
		c.Template = jsonpath.New(c.Name)
		return nil
	}
	c.Raw = exp
	sm := jsonPathRegexp.FindStringSubmatch(exp)
	if sm == nil {
		return fmt.Errorf("unexpected path string, expected a 'name1.name2' or '.name1.name2' or '{name1.name2}' or '{.name1.name2}'")
	}
	// Pick up the one expression which matched; in any case it'll be without
	// any enclosing curly braces, and without any leading ".". Then, turn it
	// into a normalized expression.
	if len(sm[1]) != 0 {
		exp = sm[1]
	} else {
		exp = sm[2]
	}
	c.Template = jsonpath.New(c.Name).AllowMissingKeys(true)
	return c.Template.Parse(fmt.Sprintf("{.%s}", exp))
}
