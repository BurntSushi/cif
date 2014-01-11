package cif

import (
	"bytes"
	"compress/gzip"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestWriter(t *testing.T) {
	r := strings.NewReader(cifSmall)
	cif, err := Read(r)
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	if err := cif.Write(buf); err != nil {
		t.Fatal(err)
	}

	cif2, err := Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cif, cif2) {
		t.Fatalf("Not equal:\n%s\n------------\n%s\n", cif, cif2)
	}
}

func TestPDBWriter(t *testing.T) {
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

	buf := new(bytes.Buffer)
	if err := cif.Write(buf); err != nil {
		t.Fatal(err)
	}

	cif2, err := Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cif, cif2) {
		t.Fatalf("Not equal:\n%s\n------------\n%s\n", cif, cif2)
	}
}
