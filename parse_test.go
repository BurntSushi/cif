package cif

import (
	"compress/gzip"
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	f, err := os.Open("/data/bio/mmcif/ct/1ctf.cif.gz")
	if err != nil {
		return // skip the test
	}
	fz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	cif, err := ReadCIF(fz)
	if err != nil {
		t.Fatal(err)
	}
	pf("%v\n", cif.blocks["1ctf"].Loops["atom_site.id"].Values[1][1])
}
