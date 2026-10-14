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

func TestListOfMapsCellsExpandPerElement(t *testing.T) {
	type row struct {
		Name string
		Gpus []map[string]interface{}
	}
	data := []row{{"a", []map[string]interface{}{
		{"b": "2", "a": "1"},
		{"a": "3"},
	}}}
	vp, err := NewCustomColumnsPrinterFromSpec("NAME:{.Name},GPUS:{.Gpus}")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := vp.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "{") || strings.Contains(got, "map[") {
		t.Fatalf("map not expanded:\n%s", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// header + a,b + blank separator + a
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines:\n%s", got)
	}
}

func TestNestedMapsAreIndented(t *testing.T) {
	type row struct {
		Name string
		Cpus map[string]interface{}
	}
	data := []row{{"a", map[string]interface{}{
		"version": "v1",
		"sockets": []map[string]interface{}{{"cores": 16, "arch": "x86_64"}},
		"meta":    map[string]interface{}{"k": "v", "inner": map[string]interface{}{"z": 1}},
		"empty":   map[string]interface{}{},
	}}}
	vp, err := NewCustomColumnsPrinterFromSpec("NAME:{.Name},CPUS:{.Cpus}")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := vp.Fprint(&buf, data); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	col := strings.Index(lines[0], "CPUS")
	var cell []string
	for _, l := range lines[1:] {
		cell = append(cell, l[col:])
	}
	want := []string{
		"empty: <none>",
		"meta:",
		"  inner:",
		"    z: 1",
		"  k: v",
		"sockets:",
		"  arch: x86_64",
		"  cores: 16",
		"version: v1",
	}
	if strings.Join(cell, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(cell, "\n"), strings.Join(want, "\n"))
	}
}
