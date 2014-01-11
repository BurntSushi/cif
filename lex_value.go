package cif

import "strings"

// lexValue tries to consume any kind of value (omitted, missing, integer,
// float or string). If a valid value cannot be found, the lexer fails.
func lexValue(lx *lexer) stateFn {
	// Get the previous character consumed, so that we can check for
	// eol and noteol.
	lx.backup()
	previous := lx.next()

	// Make sure that no reserved words are used for unquoted values.
	reserved := []string{"data_", "save_"}
	for _, word := range reserved {
		if lx.aheadMatch(word) {
			return lx.errf("%s cannot be used in the beginning of an unquoted "+
				"value.", word)
		}
	}

	r := lx.next()
	switch {
	case r == dataOmitted && (isWhiteSpace(lx.peek()) || lx.peek() == eof):
		lx.emit(itemDataOmitted)
		return lexSpaceOrEof(lx, lexValueEnd)
	case r == dataMissing:
		lx.emit(itemDataMissing)
		return lexSpaceOrEof(lx, lexValueEnd)
	case r == '+' || r == '-':
		return lexValueIntegerStart
	case isDigit(r):
		return lexValueInteger
	case r == '.':
		return lexValueFloatAfterDecimal
	case r == '\'' || r == '"':
		lx.ignore()
		return lexValueQuoted(r)
	case isNL(previous) && r == ';':
		lx.ignore()
		return lexValueTextFieldFirstLine
	case isNL(previous) && isOrdinaryChar(r):
		lx.push(lexValueUnquotedEnd)
		return lx.chars(false, isNonBlankChar)
	case !isNL(previous) && (isOrdinaryChar(r) || r == ';'):
		lx.push(lexValueUnquotedEnd)
		return lx.chars(false, isNonBlankChar)
	}
	return lx.errf("Expected a value ('.', '?', numeric or string), "+
		"but got '%s'.", r)
}

// lexValueEnd ensures there is whitespace after a value and returns to the
// next state on the stack.
func lexValueEnd(lx *lexer) stateFn {
	return lexWhiteSpace(lx, lx.pop())
}

// lexValueIntegerStart tries to consume the first digit in an integer or
// float. This is used when the '+' or '-' is seen.
func lexValueIntegerStart(lx *lexer) stateFn {
	r := lx.peek()
	switch {
	case isDigit(r):
		lx.accept(r)
		return lexValueInteger
	case r == '.':
		lx.accept(r)
		return lexValueFloatAfterDecimal
	}
	// Fall back to unquoted string.
	lx.push(lexValueUnquotedEnd)
	return lx.chars(false, isNonBlankChar)
}

// lexValueInteger tries to consume an integer while allowing for the
// possibility that the token may be a float. This assumes that the first
// digit has already been consumed.
func lexValueInteger(lx *lexer) stateFn {
	r := lx.peek()
	switch {
	case isDigit(r):
		lx.accept(r)
		return lexValueInteger
	case r == '.':
		lx.accept(r)
		return lexValueFloatAfterDecimal
	case r == 'e' || r == 'E':
		lx.accept(r)
		return lexValueExponentFirst
	case isWhiteSpace(r) || r == eof:
		lx.emit(itemDataInteger)
		return lexSpaceOrEof(lx, lexValueEnd)
	}
	// Fall back to unquoted string.
	lx.push(lexValueUnquotedEnd)
	return lx.chars(false, isNonBlankChar)
}

// lexValueExponentFirst checks for a '+' or '-' before any digits in an
// exponent.
func lexValueExponentFirst(lx *lexer) stateFn {
	r := lx.peek()
	if r == '+' || r == '-' {
		lx.accept(r)
	}
	return lexValueExponent
}

// lexValueExponent consumes the digit portion of an exponent.
func lexValueExponent(lx *lexer) stateFn {
	r := lx.peek()
	switch {
	case isDigit(r):
		lx.accept(r)
		return lexValueExponent
	case isWhiteSpace(r) || r == eof:
		lx.emit(itemDataFloat)
		return lexSpaceOrEof(lx, lexValueEnd)
	}
	// Fall back to unquoted string.
	lx.push(lexValueUnquotedEnd)
	return lx.chars(false, isNonBlankChar)
}

// lexValueExponentEnd emits the value as a float.
func lexValueExponentEnd(lx *lexer) stateFn {
	lx.emit(itemDataFloat)
	return lexSpaceOrEof(lx, lexValueEnd)
}

// lexValueFloatAfterDecimal consumes the digits after a decimal point in
// a float value.
func lexValueFloatAfterDecimal(lx *lexer) stateFn {
	r := lx.peek()
	switch {
	case isDigit(r):
		lx.accept(r)
		return lexValueFloatAfterDecimal
	case r == 'e' || r == 'E':
		lx.accept(r)
		return lexValueExponentFirst
	case isWhiteSpace(r) || r == eof:
		lx.emit(itemDataFloat)
		return lexSpaceOrEof(lx, lexValueEnd)
	}
	// Fall back to unquoted string.
	lx.push(lexValueUnquotedEnd)
	return lx.chars(false, isNonBlankChar)
}

// lexValueTextField assumes that '<eol>;' has already been consumed (and
// ignored), and parses the rest of the value.
// The first line can be a sequence of any printable character, but all
// subsequent lines may not begin with a ';'.
func lexValueTextFieldFirstLine(lx *lexer) stateFn {
	lx.push(lexValueTextField)
	return lx.chars(false, isPrintChar)
}

// lexValueTextField assumes that the first line of a semi-colon text field
// has already been parsed. Therefore, it looks for the text field terminator
// '<eol>;' but otherwises consumes more text.
func lexValueTextField(lx *lexer) stateFn {
	s := lx.peekAt(2)
	if len(s) == 2 && isNL(rune(s[0])) && s[1] == ';' {
		lx.emit(itemDataString)
		return lx.acceptStr(s, lexSpaceOrEof(lx, lexValueEnd))
	}
	if len(s) < 2 {
		return lx.errf("Expected a semi-colon terminator, but got EOF.")
	}

	r := lx.next()
	if !isNL(r) {
		lx.errf("Expected at a new line before end of semi-colon text field, "+
			"but got '%s' instead.", r)
	}
	// Cannot be a semi-colon here, handled above.
	// But if it's an <eol>, then we can just skip this line.
	if isNL(lx.peek()) {
		return lexValueTextField
	}
	lx.push(lexValueTextField)
	return lx.chars(true, isPrintChar)
}

// lexValueUnquotedEnd emits the value as a string. It also makes sure that
// no reserved words were used.
func lexValueUnquotedEnd(lx *lexer) stateFn {
	// Make sure that no reserved words are used for unquoted values.
	reserved := []string{"loop_", "stop_", "global_"}
	for _, word := range reserved {
		if word == strings.ToLower(lx.current()) {
			return lx.errf("%s cannot be used as an unquoted string value.",
				word)
		}
	}
	lx.emit(itemDataString)
	return lexSpaceOrEof(lx, lexValueEnd)
}

// lexValueQuoted consumes a quoted string. It makes sure that values like
// 'andrew's pet' are valid.
func lexValueQuoted(quote rune) stateFn {
	return func(lx *lexer) stateFn {
		for {
			r := lx.next()
			if isNL(r) {
				return lx.errf("Quoted strings may not contain new lines.")
			}
			if r == eof {
				return lx.errf("Expected end of quoted string, but got EOF.")
			}

			peek := lx.peekAt(1)
			if r == quote && (len(peek) == 0 || isWhiteSpace(rune(peek[0]))) {
				lx.backup()
				lx.emit(itemDataString)
				lx.accept(r)
				lx.ignore()
				return lexSpaceOrEof(lx, lexValueEnd)
			}
		}
	}
}
