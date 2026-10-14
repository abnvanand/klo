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

func TestMapCellsWrapKeyValuePerLine(t *testing.T) {
	type row struct {
		Name string
		Info map[string]interface{}
	}
	data := []row{{"a", map[string]interface{}{
		"tool":  "DCGM R2",
		"apSku": nil,
		"ecids": []string{"e1", "e2"},
	}}}
	vp, err := NewCustomColumnsPrinterFromSpec("NAME:{.Name},INFO:{.Info}")
	if err != nil {
		t.Fatal(err)
	}
	p := vp.(*CustomColumnsPrinter)
	var buf bytes.Buffer
	if err := p.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// header + 3 sorted entries
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines:\n%s", buf.String())
	}
	for i, want := range []string{"apSku: <none>", "ecids: e1, e2", "tool: DCGM R2"} {
		if !strings.HasSuffix(strings.TrimSpace(lines[i+1]), want) {
			t.Fatalf("line %d: want suffix %q:\n%s", i+1, want, buf.String())
		}
	}
	if strings.Contains(buf.String(), "map[") {
		t.Fatalf("map leaked as %%v:\n%s", buf.String())
	}
	buf.Reset()
	p.NoWrapSlices = true
	if err := p.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "map[") {
		t.Fatalf("expected single-line map:\n%s", buf.String())
	}
}
