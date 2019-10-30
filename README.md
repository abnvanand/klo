# klo

`klo` is a Go package for `kubectl`-like output of Go values (such as structs,
maps, et cetera) in several output formats. You might want to use this package
in your CLI tools to easily offer `kubectl`-like output formatting to your
Kubernetes-spoiled users.

[The Only True Go Way](https://golang.org/doc/effective_go.html#package-names)
mandates package names to be short, concise, evocative. Thus, the package name
`klo` was chosen to represent the essence of **`k`**`ubectl`-**l**ike
**o**utput in a manner as short, concise and evocative as ever. If you happen
to have sanitary associations, then better flush them now.

## Supported Output Formats

The following output formats are supported:

- ASCII columns, which optionally can be customized (`-o custom-columns=` and
  `-o custom-columns-file=`).
  - optional sorting by specific column(s) using JSONPath expressions.
- JSON and JSONPath-customized (`-o json`, `-o jsonpath=`, and `-o
  jsonpath-file=`).
- YAML (`-o yaml`).

> **Note:** `-o name` and `-o wide` are application-specific and are basically
> customized ASCII column formats, with just varying custom column
> configurations. Thus, they can be easily implemented in your appliaction
> itself and then use the existing `klo` package features.

## Usage

The following code example prints a table with multiple columns, and the rows
sorted by the NAME column.

```go
import (
    "os"
    "github.com/thediveo/klo"
)

type myobj struct {
    Name string
    Foo int
    Bar string
}

func main() {
	type myobj struct {
		Name string
		Foo  int
		Bar  string
	}
	// A slice of objects we want to print as a table with custom columns.
	list := []myobj{
		myobj{Name: "One", Foo: 42},
		myobj{Name: "Two", Foo: 666, Bar: "Bar"},
		myobj{Name: "Another Two", Foo: 123, Bar: "Bar"},
	}

	// Create a table printer with custom columns, to be filled from fields
	// of the objects (namely, Name, Foo, and Bar fields).
	prn, err := klo.PrinterFromFlag("", "NAME:{.Name},FOO:{.Foo},BAR:{.Bar}", "")
	if err != nil {
		panic(err)
	}
	// Use a table sorter and tell it to sort by the Name field of our column objects.
	table, err := klo.NewSortingPrinter("{.Name}", prn)
	table.Fprint(os.Stdout, list)

```

This will output:

```
NAME        FOO  BAR
Another Two 123  Bar
One         42
Two         666  Bar
```

## Copyright and License

`klo` is Copyright 2019 Harald Albrecht, and licensed under the [Apache
License, Version
2.0](https://github.com/TheDiveO/go-mntinfo/blob/master/LICENSE).
