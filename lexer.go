package cif

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	pf = fmt.Printf
	sf = fmt.Sprintf
)

type stateFn func(lx *lexer) stateFn

type lexer struct {
	input  string
	start  int
	pos    int
	width  int
	line   int
	column int
	state  stateFn
	items  chan item

	// A stack of state functions used to maintain context.
	// The idea is to reuse parts of the state machine in various places.
	// The last state on the stack is used after a value has
	// been lexed. Similarly for comments.
	stack []stateFn
}

type item struct {
	typ  itemType
	val  string
	line int
}

func lex(input string) *lexer {
	lx := &lexer{
		input:  input,
		state:  lexCifInitial,
		line:   1,
		column: 1,
		items:  make(chan item, 10),
		stack:  make([]stateFn, 0, 10),
	}
	return lx
}

func (lx *lexer) nextItem() item {
	for {
		select {
		case item := <-lx.items:
			return item
		default:
			// TODO: Remove this. Used for debugging while building lexer.
			if lx.state == nil {
				return item{itemEOF, "", 0}
			}
			lx.state = lx.state(lx)
		}
	}
	panic("not reached")
}

func (lx *lexer) push(state stateFn) {
	lx.stack = append(lx.stack, state)
}

func (lx *lexer) pop() stateFn {
	if len(lx.stack) == 0 {
		return lx.errf("BUG in lexer: no states to pop.")
	}
	last := lx.stack[len(lx.stack)-1]
	lx.stack = lx.stack[0 : len(lx.stack)-1]
	return last
}

func (lx *lexer) current() string {
	return lx.input[lx.start:lx.pos]
}

func (lx *lexer) emit(typ itemType) {
	lx.items <- item{typ, lx.current(), lx.line}
	lx.start = lx.pos
}

func (lx *lexer) next() (r rune) {
	if lx.pos >= len(lx.input) {
		lx.width = 0
		return eof
	}
	if lx.input[lx.pos] == '\n' {
		lx.line++
		lx.column = 1
	}
	// This is wrong. The CIF format says that only a subset of ASCII
	// is allowed in CIF files.
	r, lx.width = utf8.DecodeRuneInString(lx.input[lx.pos:])
	lx.pos += lx.width
	return r
}

// ignore skips over the pending input before this point.
func (lx *lexer) ignore() {
	lx.start = lx.pos
}

// backup steps back one rune. Can be called only once per call of next.
func (lx *lexer) backup() {
	lx.pos -= lx.width
	if lx.pos < len(lx.input) && lx.input[lx.pos] == '\n' {
		lx.line--
		// column is now wrong, but maybe that's OK.
	}
}

// accept consumes the next rune if it's equal to `valid`.
func (lx *lexer) accept(valid rune) bool {
	if lx.next() == valid {
		return true
	}
	lx.backup()
	return false
}

// acceptStr consumes the string given. If the string consumed does not match,
// then the lexer fails. Otherwise, the string consumed is thrown away and
// lexing moves on to the state given.
func (lx *lexer) acceptStr(s string, next stateFn) stateFn {
	for _, r := range s {
		if !lx.accept(r) {
			return lx.errf("Expected '%s' but got '%s' instead (in '%s').",
				r, lx.peek(), s)
		}
	}
	lx.ignore()
	return next
}

// peek returns but does not consume the next rune in the input.
func (lx *lexer) peek() rune {
	r := lx.next()
	lx.backup()
	return r
}

// peekAt returns the string (indexed by rune) from the current position
// up to the length given. This does not consume input.
// If the length given exceeds what's left in the input, then the rest of the
// input is returned.
func (lx *lexer) peekAt(length int) string {
	if lx.pos >= len(lx.input) {
		return ""
	}
	count, realLength := 0, 0
	for i := range lx.input[lx.pos:] {
		if count == length {
			break
		}
		count++
		realLength = i + 1
	}
	return lx.input[lx.pos : lx.pos+realLength]
}

// aheadMatch looks ahead from the current lex position to see if the next
// len(s) characters match s (case insensitive).
func (lx *lexer) aheadMatch(s string) bool {
	return strings.ToLower(lx.peekAt(len(s))) == strings.ToLower(s)
}

// errf stops all lexing by emitting an error and returning `nil`.
// Note that any value that is a character is escaped if it's a special
// character (new lines, tabs, etc.).
func (lx *lexer) errf(format string, values ...interface{}) stateFn {
	for i, value := range values {
		if v, ok := value.(rune); ok {
			switch v {
			case '\n':
				values[i] = "\\n"
			case 0:
				values[i] = "EOF"
			default:
				values[i] = string(v)
			}
		}
	}
	lx.items <- item{
		itemError,
		sf(format, values...),
		lx.line,
	}
	return nil
}

func (lx *lexer) stop() stateFn {
	lx.ignore()
	lx.emit(itemEOF)
	return nil
}
