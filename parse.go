package cif

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

type parser struct {
	*CIF
	lx   *lexer
	line int
}

type cifError string

func (ce cifError) Error() string {
	return string(ce)
}

// Read reads CIF formatted input and returns a CIF value if and only if the
// input conforms to the CIF 1.1 specification.
func Read(r io.Reader) (*CIF, error) {
	cif := &CIF{
		Version: "",
		Blocks:  make(map[string]*DataBlock, 10),
	}
	input, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return (&parser{CIF: cif}).parse(string(input))
}

func (p *parser) errf(format string, v ...interface{}) {
	err := fmt.Sprintf(format, v...)
	panic(cifError(fmt.Sprintf("CIF parse error (line %d): %s", p.line, err)))
}

func (p *parser) next() item {
	t := p.lx.nextItem()
	if t.typ == itemComment {
		return p.next()
	}
	if t.typ == itemError {
		p.errf("%s", t.val)
	}
	p.line = t.line
	return t
}

func (p *parser) parse(input string) (_ *CIF, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(cifError); ok {
				err = e
				return
			}
		}
	}()
	p.lx = lex(input)
	for t := p.next(); ; {
		switch t.typ {
		case itemEOF:
			return p.CIF, nil
		case itemDataBlockStart:
			t = p.parseDataBlock(strings.ToLower(t.val))
		case itemVersion:
			p.Version = t.val[3:]
			t = p.next()
		default:
			p.errf("Expected comments, whitespace or a data block heading, "+
				"but got a '%s' instead.", t.typ)
		}
	}
	panic("unreachable")
}

func (p *parser) parseDataBlock(name string) item {
	if _, ok := p.Blocks[name]; ok {
		p.errf("Data block with name '%s' already exists.", name)
	}
	dblock := &DataBlock{
		Block: Block{
			Name:  name,
			Items: make(map[string]Value, 10),
			Loops: make(map[string]*Loop, 5),
		},
		Frames: make(map[string]*SaveFrame, 0), // only used for dictionaries
	}
	p.Blocks[name] = dblock
	for t := p.next(); ; {
		switch t.typ {
		case itemEOF, itemDataBlockStart:
			return t
		case itemSaveFrameStart:
			p.parseSaveFrame(dblock, strings.ToLower(t.val))
			t = p.next()
		case itemLoop:
			t = p.parseLoop(dblock.Block)
		case itemDataTag:
			p.parseItemValue(dblock.Block, strings.ToLower(t.val))
			t = p.next()
		default:
			p.errf("Expected comments, whitespace or a data block heading, "+
				"but got a '%s' instead.", t.typ)
		}
	}
	panic("unreachable")
}

func (p *parser) parseSaveFrame(dblock *DataBlock, name string) {
	if _, ok := dblock.Frames[name]; ok {
		p.errf("Save frame with name '%s' already exists in data block '%s'.",
			name, dblock.Name)
	}
	frame := &SaveFrame{
		Block: Block{
			Name:  name,
			Items: make(map[string]Value, 10),
			Loops: make(map[string]*Loop, 5),
		},
	}
	dblock.Frames[name] = frame
	for t := p.next(); ; {
		switch t.typ {
		case itemSaveFrameEnd:
			return
		case itemLoop:
			t = p.parseLoop(frame.Block)
		case itemDataTag:
			p.parseItemValue(frame.Block, strings.ToLower(t.val))
			t = p.next()
		default:
			p.errf("Expected a data item or end of save frame delimiter, "+
				"but got a '%s' instead.", t.typ)
		}
	}
}

func (p *parser) parseItemValue(b Block, name string) {
	p.assertUniqueTag(b, name)
	b.Items[name] = p.parseValue(p.next(), b.Name, name)
}

func (p *parser) parseValue(t item, bname, name string) Value {
	switch t.typ {
	case itemDataOmitted:
		return AsValue(".")
	case itemDataMissing:
		return AsValue("?")
	case itemDataInteger:
		n, err := strconv.Atoi(t.val)
		if err != nil {
			p.errf("Could not parse '%s' as integer: %s", t.val, err)
		}
		return AsValue(n)
	case itemDataFloat:
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			p.errf("Could not parse '%s' as float: %s", t.val, err)
		}
		return AsValue(f)
	case itemDataString:
		return AsValue(t.val)
	default:
		p.errf("Expected value for data tag '%s' in block '%s', but got a "+
			"'%s' instead.", name, bname, t.typ)
	}
	panic("unreachable")
}

func (p *parser) assertUniqueTag(b Block, name string) {
	_, iok := b.Items[name]
	_, lok := b.Loops[name]
	if iok || lok {
		p.errf("Data item with name '%s' already exists in block '%s'.",
			name, b.Name)
	}
}

func isValueType(t itemType) bool {
	return t == itemDataOmitted || t == itemDataMissing ||
		t == itemDataInteger || t == itemDataFloat || t == itemDataString
}

func isNull(t itemType) bool {
	return t == itemDataOmitted || t == itemDataMissing
}

func isInteger(t itemType) bool {
	return t == itemDataInteger
}

func isFloat(t itemType) bool {
	return t == itemDataFloat
}
