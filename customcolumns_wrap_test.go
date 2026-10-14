package klo

import (
	"bytes"
	"strings"
	"testing"
)

func TestSliceCellsWrapOnePerLine(t *testing.T) {
	type row struct {
		Name string
		OS   []string
	}
	data := []row{{"a", []string{"x1", "x2", "x3"}}, {"b", nil}}
	vp, err := NewCustomColumnsPrinterFromSpec("NAME:{.Name},OS:{.OS}")
	if err != nil {
		t.Fatal(err)
	}
	p := vp.(*CustomColumnsPrinter)
	var buf bytes.Buffer
	if err := p.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "[") || !strings.Contains(got, "x2\n") || !strings.Contains(got, "x3\n") {
		t.Fatalf("unexpected output:\n%s", got)
	}
	buf.Reset()
	p.NoWrapSlices = true
	if err := p.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[x1 x2 x3]") {
		t.Fatalf("expected single-line slice:\n%s", buf.String())
	}
}
