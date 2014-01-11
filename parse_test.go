package cif

import (
	"compress/gzip"
	"flag"
	"os"
	"strings"
	"testing"
)

var flagDev = false

func init() {
	flag.BoolVar(&flagDev, "dev", flagDev,
		"When set, runs tests on some data files. Uses hard-coded paths.")
}

func TestParser(t *testing.T) {
	r := strings.NewReader(cifSmall)
	cif, err := Read(r)
	if err != nil {
		t.Fatal(err)
	}
	if cif.Version != "CIF_1.1" {
		t.Fatalf("Version mismatch. Should be 'CIF_1.1' but is '%s'.",
			cif.Version)
	}
}

func TestParsePDB(t *testing.T) {
	if !flagDev {
		return
	}

	f, err := os.Open("/data/bio/mmcif/ct/1ctf.cif.gz")
	if err != nil {
		return // skip the test
	}
	fz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	cif, err := Read(fz)
	if err != nil {
		t.Fatal(err)
	}
	block := cif.Blocks["1ctf"]
	xs := block.Loops["atom_site.cartn_x"].Get("atom_site.cartn_x").Floats()
	pf("%v\n", xs[0])
}

func TestParseDict(t *testing.T) {
	if !flagDev {
		return
	}

	f, err := os.Open("/home/andrew/tmp/mmcif_pdbx_v40.dic")
	if err != nil {
		return // skip the test
	}
	cif, err := Read(f)
	if err != nil {
		t.Fatal(err)
	}
	block := cif.Blocks["mmcif_pdbx.dic"]

	frame := block.Frames["_atom_site.cartn_x"]
	pf("%v\n", frame.Items["item_type.code"])

	frame = block.Frames["_entity_src_gen.pdbx_seq_type"]
	pf("%v\n", frame.Items["item_type.code"])
}
