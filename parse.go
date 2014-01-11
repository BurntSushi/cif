package cif

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

type CIF struct {
	Version string
	blocks  map[string]dataBlock
	lx      *lexer
	line    int
}

type block struct {
	Name  string
	Items map[string]Value
	Loops map[string]loop
}

type dataBlock struct {
	block
	Frames map[string]saveFrame
}

type saveFrame struct {
	block
}

type loop struct {
	Columns map[string]int
	Values  [][]Value
}

type Value interface{}

type cifError string

func (ce cifError) Error() string {
	return string(ce)
}

func ReadCIF(r io.Reader) (*CIF, error) {
	cif := &CIF{
		Version: "",
		blocks:  make(map[string]dataBlock, 10),
	}
	input, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return cif.parse(string(input))
}

func (cif *CIF) errf(format string, v ...interface{}) {
	err := fmt.Sprintf(format, v...)
	panic(cifError(fmt.Sprintf("CIF parse error (line %d): %s", cif.line, err)))
}

func (cif *CIF) next() item {
	t := cif.lx.nextItem()
	// pf("%s :: %s (%d)\n", t.typ, t.val, t.line) 
	if t.typ == itemComment {
		return cif.next()
	}
	cif.line = t.line
	return t
}

func (cif *CIF) parse(input string) (_ *CIF, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(cifError); ok {
				err = e
				return
			}
		}
	}()
	cif.lx = lex(input)
	for t := cif.next(); ; {
		switch t.typ {
		case itemEOF:
			return cif, nil
		case itemDataBlockStart:
			t = cif.parseDataBlock(strings.ToLower(t.val))
		default:
			cif.errf("Expected comments, whitespace or a data block heading, "+
				"but got a '%s' instead.", t.typ)
		}
	}
	panic("unreachable")
}

func (cif *CIF) parseDataBlock(name string) item {
	if _, ok := cif.blocks[name]; ok {
		cif.errf("Data block with name '%s' already exists.", name)
	}
	dblock := dataBlock{
		block: block{
			Name:  name,
			Items: make(map[string]Value, 10),
			Loops: make(map[string]loop, 5),
		},
		Frames: make(map[string]saveFrame, 0), // only used for dictionaries
	}
	cif.blocks[name] = dblock
	for t := cif.next(); ; {
		switch t.typ {
		case itemEOF, itemDataBlockStart:
			return t
		case itemSaveFrameStart:
			cif.parseSaveFrame(dblock, strings.ToLower(t.val))
			t = cif.next()
		case itemLoop:
			t = cif.parseLoop(dblock.block)
		case itemDataTag:
			cif.parseItemValue(dblock.block, strings.ToLower(t.val))
			t = cif.next()
		default:
			cif.errf("Expected comments, whitespace or a data block heading, "+
				"but got a '%s' instead.", t.typ)
		}
	}
	panic("unreachable")
}

func (cif *CIF) parseSaveFrame(dblock dataBlock, name string) {
	if _, ok := dblock.Frames[name]; ok {
		cif.errf("Save frame with name '%s' already exists in data block '%s'.",
			name, dblock.Name)
	}
	frame := saveFrame{
		block: block{
			Name:  name,
			Items: make(map[string]Value, 10),
			Loops: make(map[string]loop, 5),
		},
	}
	dblock.Frames[name] = frame
	for t := cif.next(); ; {
		switch t.typ {
		case itemSaveFrameEnd:
			return
		case itemLoop:
			t = cif.parseLoop(frame.block)
		case itemDataTag:
			cif.parseItemValue(frame.block, strings.ToLower(t.val))
			t = cif.next()
		default:
			cif.errf("Expected a data item or end of save frame delimiter, "+
				"but got a '%s' instead.", t.typ)
		}
	}
}

func (cif *CIF) parseLoop(b block) item {
	loopLine := cif.line
	t := cif.next()
	if t.typ != itemDataTag {
		cif.errf("After 'loop_' declaration, there must be at least one "+
			"data tag, but found '%s' instead.", t.typ)
	}
	loop := loop{
		Columns: make(map[string]int, 5),
		Values:  nil,
	}
	var columns []string
	for i := 0; t.typ == itemDataTag; t, i = cif.next(), i+1 {
		name := strings.ToLower(t.val)
		cif.assertUniqueTag(b, name)
		loop.Columns[name] = i
		columns = append(columns, name)
	}

	loop.Values = make([][]Value, len(loop.Columns))
	if !isValueType(t.typ) {
		cif.errf("After 'loop_' declaration, there must be at least one "+
			"data tag and at least one value, but found '%s' instead of a "+
			"value.", t.typ)
	}
	count := 0 // must end up being a multiple of len(loop.Columns)
	for i := 0; isValueType(t.typ); t, i, count = cif.next(), i+1, count+1 {
		column := i % len(loop.Columns)
		v := cif.parseValue(t, b.Name, columns[column])
		loop.Values[column] = append(loop.Values[column], v)
	}
	if count % len(loop.Columns) != 0 {
		cif.errf("There are %d values in loop (starting on line %d), which is "+
			"not a multiple of the number of columns in the loop (%d).",
			count, loopLine, len(loop.Columns))
	}
	for tag := range loop.Columns {
		b.Loops[tag] = loop
	}
	return t
}

func (cif *CIF) parseItemValue(b block, name string) {
	cif.assertUniqueTag(b, name)
	b.Items[name] = cif.parseValue(cif.next(), b.Name, name)
}

func (cif *CIF) parseValue(t item, bname, name string) Value {
	switch t.typ {
	case itemDataOmitted:
		return "."
	case itemDataMissing:
		return "?"
	case itemDataInteger:
		n, err := strconv.Atoi(t.val)
		if err != nil {
			cif.errf("Could not parse '%s' as integer: %s", t.val, err)
		}
		return n
	case itemDataFloat:
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			cif.errf("Could not parse '%s' as float: %s", t.val, err)
		}
		return f
	case itemDataString:
		return t.val
	default:
		cif.errf("Expected value for data tag '%s' in block '%s', but got a "+
			"'%s' instead.", name, bname, t.typ)
	}
	panic("unreachable")
}

func (cif *CIF) assertUniqueTag(b block, name string) {
	_, iok := b.Items[name]
	_, lok := b.Loops[name]
	if iok || lok {
		cif.errf("Data item with name '%s' already exists in block '%s'.",
			name, b.Name)
	}
}

func isValueType(t itemType) bool {
	return t == itemDataOmitted || t == itemDataMissing ||
		t == itemDataInteger || t == itemDataFloat || t == itemDataString
}
