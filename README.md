Package cif provides a reader and a writer for the Crystallographic Information
File (CIF) format. This package corresponds (almost) exactly to version 1.1
of the CIF specification:
http://www.iucr.org/resources/cif/spec/version1.1/cifsyntax

This package should conform to the entirety of the specification, except it
does not enforce maximum line length limits while reading or writing CIF files.
This behavior may be changed in the future.


### Installation

If you have Go installed and
[your GOPATH is setup](http://golang.org/doc/code.html#GOPATH), then `cif` can 
be installed with `go get`:

    go get github.com/BurntSushi/cif

And you can run tests with:

    go test github.com/BurntSushi/cif


### Documentation

Documentation is available at
[godoc.org/github.com/BurntSushi/cif](http://godoc.org/github.com/BurntSushi/cif).


### Example

Here's a simple example parsing a small section of a PDB file:

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/BurntSushi/cif"
)

func main() {
	data := `#\\#CIF_1.1
data_1CTF
_entry.id  1CTF
loop_
_entity_poly_seq.num 
_entity_poly_seq.mon_id 
1  ALA
2  ALA
3  GLU
4  GLU
5  LYS
`
	cif, err := cif.Read(strings.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	block := cif.Blocks["1ctf"] // all items are stored in lowercase
	fmt.Printf("%s\n", block.Name)

	// You can retrieve a loop by using any of the data tags defined in
	// the loop.
	loop := block.Loops["entity_poly_seq.num"]

	// While using the same key twice may seem redundant, this approach
	// guarantees that you're selecting values from precisely the same table.
	// Also, loop.Get is guaranteed to return a []string, []int or []float64.
	seqNums := loop.Get("entity_poly_seq.num").Ints()
	residues := loop.Get("entity_poly_seq.mon_id").Strings()

	// If the access methods fail, then nil is returned.
	if seqNums == nil {
		log.Fatal("Could not read sequence numbers as integers.")
	}
	if residues == nil {
		log.Fatal("Could not read residues as strings.")
	}

	// All columns in a table are guaranteed by Read to have the same length.
	for i := 0; i < len(seqNums); i++ {
		fmt.Printf("%d %s\n", seqNums[i], residues[i])
	}
	// Output:
	// 1ctf
	// 1 ALA
	// 2 ALA
	// 3 GLU
	// 4 GLU
	// 5 LYS
}
```
