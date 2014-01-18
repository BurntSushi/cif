package cif

import "fmt"

var (
	pf = fmt.Printf
	sf = fmt.Sprintf
)

type stateFn func(lx *lexer) stateFn

type lexer struct {
	input string
	start int
	pos   int
	width int
	line  int
	state stateFn
	emitted *item

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
		input: input,
		state: lexCifInitial,
		line:  1,
		emitted: nil,
		stack: make([]stateFn, 0, 10),
	}
	return lx
}

func (lx *lexer) nextItem() (it item) {
	for lx.emitted == nil && lx.state != nil {
		lx.state = lx.state(lx)
	}
	if lx.state == nil {
		return item{itemEOF, "", 0}
	}
	it, lx.emitted = *lx.emitted, nil
	return it
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

var emitted = &item{}

func (lx *lexer) emit(typ itemType) {
	if lx.emitted != nil {
		panic("BUG in lexer: a state may only emit a single token")
	}
	lx.emitted = emitted
	lx.emitted.typ = typ
	lx.emitted.val = lx.current()
	lx.emitted.line = lx.line
	lx.start = lx.pos
}

func (lx *lexer) next() (r rune) {
	if lx.pos >= len(lx.input) {
		lx.width = 0
		return eof
	}
	if lx.input[lx.pos] == '\n' {
		lx.line++
	}
	// We're allowed to do this because the CIF format only permits
	// ASCII characters.
	r = rune(lx.input[lx.pos])
	lx.width = 1
	lx.pos++
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
	if lx.pos >= len(lx.input) {
		return eof
	}
	return rune(lx.input[lx.pos])
}

// peekAt returns the string (indexed by byte) from the current position
// up to the length given. This does not consume input.
// If the length given exceeds what's left in the input, then the rest of the
// input is returned.
func (lx *lexer) peekAt(length int) string {
	if lx.pos >= len(lx.input) {
		return ""
	}
	upto := lx.pos + length
	if upto > len(lx.input) {
		upto = len(lx.input)
	}
	return lx.input[lx.pos:upto]
}

// aheadMatch looks ahead from the current lex position to see if the next
// len(s) characters match s (case insensitive).
func (lx *lexer) aheadMatch(s string) bool {
	return lx.peekAt(len(s)) == s
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
	lx.emitted = &item{
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
