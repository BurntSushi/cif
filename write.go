package cif

import (
	"fmt"
	"io"
	"regexp"
)

var (
	matchNumeric1 = regexp.MustCompile(
		"(\\+|-)?[0-9]+(\\.[0-9]*([eE](\\+|-)?[0-9]+)?)?")
	matchNumeric2 = regexp.MustCompile(
		"(\\+|-)?\\.[0-9]+([eE](\\+|-)?[0-9]+)?")
)

type writeError string

func (we writeError) Error() string {
	return string(we)
}

type writer struct {
	*CIF
	w io.Writer
}

// Write writes an existing CIF to the writer given.
// It is appropriate to read a CIF with Read, modify it in place, and then
// call Write.
// Note that Write does not currently impose an ordering on the data items
// written (except for the columns in a table).
func (cif *CIF) Write(w io.Writer) error {
	return writer{cif, w}.write()
}

func (w writer) errf(format string, v ...interface{}) {
	err := fmt.Sprintf(format, v...)
	panic(writeError(fmt.Sprintf("CIF write: %s", err)))
}

func (w writer) pf(format string, v ...interface{}) {
	if _, err := fmt.Fprintf(w.w, format, v...); err != nil {
		w.errf("%s", err)
	}
}

func (w writer) write() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(cifError); ok {
				err = e
				return
			}
		}
	}()
	if len(w.Version) > 0 {
		w.pf("#\\#%s\n", w.Version)
	}
	for _, b := range w.Blocks {
		w.writeDataBlock(b)
	}
	return nil
}

func (w writer) writeDataBlock(b *DataBlock) {
	w.pf("data_%s\n", b.Name)
	for _, frame := range b.Frames {
		w.pf("save_%s\n", frame.Name)
		w.writeBlock(frame.Block)
		w.pf("save_\n")
	}
	w.writeBlock(b.Block)
}

func (w writer) writeBlock(b Block) {
	for tag, val := range b.Items {
		w.pf("_%s    %s\n", tag, w.valToStr(val))
	}

	written := make([]*Loop, 0, 10)
	for _, lp := range b.Loops {
		if loopWritten(written, lp) {
			continue
		}
		w.writeLoop(lp)
		written = append(written, lp)
	}
}

func (w writer) writeLoop(lp *Loop) {
	w.pf("loop_\n")

	order := make(map[int]string, len(lp.Columns))
	for column, i := range lp.Columns {
		order[i] = column
	}
	for i := 0; i < len(lp.Values); i++ {
		w.pf("_%s\n", order[i])
	}
	strs := make([][]string, len(lp.Values))
	for i := range lp.Values {
		switch vals := lp.Values[i].(type) {
		case cifStrings:
			strs[i] = make([]string, len(vals))
			for j, val := range vals {
				strs[i][j] = w.formatStr(val)
			}
		case cifInts:
			strs[i] = make([]string, len(vals))
			for j, val := range vals {
				strs[i][j] = fmt.Sprintf("%d", val)
			}
		case cifFloats:
			strs[i] = make([]string, len(vals))
			for j, val := range vals {
				strs[i][j] = fmt.Sprintf("%f", val)
			}
		}
	}
	for row := 0; row < len(strs[0]); row++ {
		before := ""
		for column := 0; column < len(strs); column++ {
			w.pf("%s%s", before, strs[column][row])
			before = "  "
		}
		w.pf("\n")
	}
}

func (w writer) valToStr(v Value) string {
	switch v := v.(type) {
	case cifString:
		return w.formatStr(string(v))
	case cifInt:
		return fmt.Sprintf("%d", v)
	case cifFloat:
		return fmt.Sprintf("%f", v)
	default:
		w.errf("CIF does not support value of type '%T'.", v)
	}
	panic("unreachable")
}

// formatStr examines the contents of the given string to determine how to
// write it. It decides between unquoted, single-quoted, double-quoted or
// a semi-colon text field. The first three are only used for strings without
// new lines. Semi-colon text fields are used otherwise. Quoted values are used
// when a string contains quotation marks. If a string contains both ' and ",
// then a semi-colon text field is used.
func (w writer) formatStr(s string) string {
	// We know this is a string, but if it looks numeric, quote it.
	if matchNumeric1.MatchString(s) || matchNumeric2.MatchString(s) {
		return "\"" + s + "\""
	}

	// N.B. We used some functions from the lexer for convenience.
	which := "unquoted"
	seenDouble, seenSingle := false, false
LOOP:
	for _, r := range s {
		switch {
		case isNL(r):
			which = "text"
			break LOOP
		case r == '"':
			seenDouble = true
			// If we've already seen single, then just go to text field.
			if seenSingle {
				which = "text"
				break LOOP
			}
			which = "single"
		case r == '\'':
			seenSingle = true
			// If we've already seen double, then just go to text field.
			if seenDouble {
				which = "text"
				break LOOP
			}
			which = "double"
		case !isOrdinaryChar(r):
			if seenDouble && seenSingle {
				which = "text"
				break LOOP
			} else if seenDouble {
				which = "single"
			} else { // if both are false or if only seenSingle is true
				which = "double"
			}
		case !isPrintChar(r):
			w.errf("The character '%c' is not a valid printable character "+
				"in the CIF 1.1 specification.", r)
		}
	}
	switch which {
	case "unquoted":
		return s
	case "single":
		return "'" + s + "'"
	case "double":
		return "\"" + s + "\""
	case "text":
		return "\n;" + s + "\n;"
	}
	panic(sf("unreachable: (unknown string format type '%s')", which))
}

// loopWritten returns true if the loop given has already been written for
// a particular block.
func loopWritten(written []*Loop, test *Loop) bool {
	for _, lp := range written {
		if loopEqual(lp, test) {
			return true
		}
	}
	return false
}

// loopEqual tests whether lp1 and lp2 are the same loop. This assumes that
// both `lp1` and `lp2` were declared in the same block (either data or save).
// If loops are declared in different blocks, then by definition they are not
// equal.
//
// Also, since a data tag may only appear once in each block, we need only
// discover one common tag between two loops to conclude that they are equal.
func loopEqual(lp1, lp2 *Loop) bool {
	for tag := range lp1.Columns {
		if _, ok := lp2.Columns[tag]; ok {
			return true
		}
	}
	return false
}
