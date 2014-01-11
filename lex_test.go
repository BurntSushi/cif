package cif

import "testing"

var cifSmall = `#\#CIF_1.1
data_1CTF
_entry.id 1ctf
_entry.name 'andrew's pet'
loop_ _a _b 
1
;2
;
data_abcd
save_wat
_entry.id .
_entry.name .
save_`

func TestLexer(t *testing.T) {
	if !flagDev {
		return
	}

	lx := lex(cifSmall)
	for {
		item := lx.nextItem()
		if item.typ == itemEOF {
			break
		} else if item.typ == itemError {
			t.Fatalf("Line %d: %s", item.line, item.val)
		}
		pf("%s :: %s\n", item.typ, item.val)
	}
}
