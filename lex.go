package cif

import "strings"

type itemType int

const (
	itemError itemType = iota
	itemEOF
	itemVersion
	itemComment
	itemGlobal
	itemStop
	itemDataBlockStart
	itemSaveFrameStart
	itemSaveFrameEnd
	itemLoop
	itemDataTag
	itemDataOmitted
	itemDataMissing
	itemDataInteger
	itemDataFloat
	itemDataString
	itemDataNone // used in parser to indicate no type
)

const (
	eof          = 0
	tagPrefix    = '_'
	commentStart = '#'
	dataOmitted  = '.'
	dataMissing  = '?'
)

// lexCifInitial starts consuming a CIF file. It checks for the special
// version annotation recommended in the CIF 1.1 spec.
func lexCifInitial(lx *lexer) stateFn {
	r := lx.next()
	if r == commentStart {
		return lexVersion
	}
	lx.backup()
	return lexCif
}

// lexCif consumes the top-level structure of a CIF file. It assumes that
// the version annotation has already been consumed (if present). It consumes
// input until the first data block is seen.
func lexCif(lx *lexer) stateFn {
	r := lx.peek()
	if r == eof {
		return lx.stop()
	}

	if r == commentStart || isWhiteSpace(r) {
		lx.push(lexCif)
		lx.ignore()
		return lexWhiteSpaceContinue
	}
	if s := lx.peekAt(5); strings.ToLower(s) == "data_" {
		lx.push(lexDataBlockHeading)
		return lx.acceptStr(s, lx.chars(true, isNonBlankChar))
	}
	return nil
}

// lexDataBlockHeading emits the consumed input as a data block heading, and
// makes sure there is whitespace (or EOF) after the heading.
func lexDataBlockHeading(lx *lexer) stateFn {
	lx.emit(itemDataBlockStart)
	return lexSpaceOrEof(lx, lexDataBlocks)
}

// lexSaveFrameHeading emits the consumed input as a data block heading, and
// makes sure there is whitespace (or EOF) after the heading.
func lexSaveFrameHeading(lx *lexer) stateFn {
	lx.emit(itemSaveFrameStart)
	return lexWhiteSpace(lx, lexFirstSaveDataItem)
}

// lexSaveFrameEnd consumes the end delimiter of a save frame.
func lexSaveFrameEnd(lx *lexer) stateFn {
	if s := lx.peekAt(5); strings.ToLower(s) == "save_" {
		lx.emit(itemSaveFrameEnd)
		return lx.acceptStr(s, lexSpaceOrEof(lx, lexDataBlocks))
	}
	return lx.errf("Expected 'save_' at end of save frame, but got '%s' "+
		"instead.", lx.peekAt(5))
}

// lexDataBlocks assumes that at least one whitespace character has been
// consumed, and starts looking for the next data block. If the next data
// block does not exist, then there must be either a data item or a save frame.
func lexDataBlocks(lx *lexer) stateFn {
	if r := lx.peek(); isWhiteSpace(r) {
		return lexWhiteSpace(lx, lexDataBlocks)
	} else if r == eof {
		return lx.stop()
	}

	// Check to make sure that no reserved names are being used.
	if lx.aheadMatch("global_") {
		return lx.errf("global_ is not supported in the CIF format.")
	}
	if lx.aheadMatch("stop_") {
		return lx.errf("stop_ is not supported in the CIF format.")
	}

	if s := lx.peekAt(5); strings.ToLower(s) == "data_" {
		lx.push(lexDataBlockHeading)
		return lx.acceptStr(s, lx.chars(true, isNonBlankChar))
	}
	if s := lx.peekAt(5); strings.ToLower(s) == "save_" {
		lx.push(lexSaveFrameHeading)
		return lx.acceptStr(s, lx.chars(true, isNonBlankChar))
	}
	lx.push(lexDataBlocks)
	return lexDataItem
}

// lexFirstSaveDataItem consumes one data item or else the lexer fails.
func lexFirstSaveDataItem(lx *lexer) stateFn {
	lx.push(lexSaveDataItems)
	return lexDataItem
}

// lexSaveDataItems tries to consume the next data item after one has already
// been consumed. It checks for the end 'save_' delimiter as well.
func lexSaveDataItems(lx *lexer) stateFn {
	if lx.aheadMatch("save_") {
		return lexSaveFrameEnd(lx)
	}
	if lx.peek() == tagPrefix || lx.aheadMatch("loop_") {
		lx.push(lexSaveDataItems)
		return lexDataItem
	}
	return lx.errf("Expected either a data item or the end of a save frame, "+
		"but got '%s' instead.", lx.peek())
}

// lexDataItem consumes a single data item (including its corresponding
// values(s).)
func lexDataItem(lx *lexer) stateFn {
	if s := lx.peekAt(5); strings.ToLower(s) == "loop_" {
		lx.ignore()
		lx.emit(itemLoop)
		return lx.acceptStr(s, lexLoopStartWhiteSpace)
	}
	if r := lx.next(); r != tagPrefix {
		return lx.errf("Expected data item name starting with '%s' but got "+
			"'%s' instead. (Strings with spaces must be quoted and all data "+
			"keys must begin with an underscore.)", tagPrefix, r)
	}
	lx.ignore()
	lx.push(lexValue)
	lx.push(lexDataTag)
	return lx.chars(true, isNonBlankChar)
}

// lexLoopStartWhiteSpace enforces whitespace after the initial 'loop_'.
func lexLoopStartWhiteSpace(lx *lexer) stateFn {
	return lexWhiteSpace(lx, lexLoopStart)
}

// lexLoopStart consumes one data tag, or else the lexer fails.
func lexLoopStart(lx *lexer) stateFn {
	if r := lx.next(); r != tagPrefix {
		return lx.errf("Every 'loop_' section must have at least one data "+
			"tag defined (starting with a '_'), but found '%s' instead.", r)
	}
	lx.ignore()
	lx.push(lexLoopTags)
	lx.push(lexDataTag)
	return lx.chars(true, isNonBlankChar)
}

// lexLoopTags attempts to consume a data tag, otherwise it starts consuming
// data values.
func lexLoopTags(lx *lexer) stateFn {
	if r := lx.peek(); r == tagPrefix {
		lx.next()
		lx.ignore()
		lx.push(lexLoopTags)
		lx.push(lexDataTag)
		return lx.chars(true, isNonBlankChar)
	}
	return lexLoopStartValue
}

// lexLoopStartValue consumes one value, or else the lexer fails.
func lexLoopStartValue(lx *lexer) stateFn {
	lx.push(lexLoopValues)
	return lexValue
}

// lexLoopValues attempts to consume a data value, but also checks if the
// values have ended by seeing a '_', 'data_' or 'save_'.
func lexLoopValues(lx *lexer) stateFn {
	r := lx.peek()
	peek := lx.peekAt(5)
	if r == tagPrefix {
		return lx.pop()
	}
	if r == 'd' || r == 's' || r == 'l' {
		if peek == "data_" || peek == "save_" || peek == "loop_" {
			return lx.pop()
		}
	}
	if r == eof {
		return lx.stop()
	}
	lx.push(lexLoopValues)
	return lexValue
}

// lexDataTag emits a data tag an ensures whitespace follows it.
func lexDataTag(lx *lexer) stateFn {
	lx.emit(itemDataTag)
	return lexWhiteSpace(lx, lx.pop())
}

var pred func(rune) bool

func lexOneOrMore(lx *lexer) stateFn {
	r := lx.next()
	if !pred(r) {
		return lx.errf("Expected at least one character, but "+
			"got '%s' instead.", r)
	}
	return lexPred
}

func lexPred(lx *lexer) stateFn {
	for {
		r := lx.next()
		if !pred(r) {
			lx.backup()
			return lx.pop()
		}
		return lexPred
	}
}

// chars consumes a sequence of characters while `pred` is true. If `oneOrMore`
// is true, then at least one character must match `pred`, or else the lexer
// will fail.
func (lx *lexer) chars(oneOrMore bool, predFn func(rune) bool) stateFn {
	pred = predFn
	if oneOrMore {
		return lexOneOrMore
	}
	return lexPred
}

// lexVersion attempts to lex the first 11 bytes as the string "#\#CIF_1.1".
// If it fails, it drops into a regular comment.
// This assumes that the initial '#' has already been consumed.
func lexVersion(lx *lexer) stateFn {
	letters := []rune{'\\', '#', 'C', 'I', 'F', '_', '1', '.', '1'}
	for _, letter := range letters {
		r := lx.next()
		if r != letter {
			lx.push(lexCif)
			return lexComment
		}
	}
	lx.emit(itemVersion)
	return lexCif
}

// lexSpaceOrEof ensures that the next character is either whitespace or EOF.
// If it's neither, then the lexer fails. If it's whitespace, then the lexer
// moves to the `next` state. If it's EOF, then the lexer stops successfully.
// (Note that EOF here doesn't necessarily imply a valid CIF file! It's up to
// the parser to determine that.)
func lexSpaceOrEof(lx *lexer, next stateFn) stateFn {
	lx.push(next)
	return lexSpaceOrEofContinue
}

func lexSpaceOrEofContinue(lx *lexer) stateFn {
	r := lx.peek()
	if !isWhiteSpace(r) && r != eof {
		return lx.errf("Expected whitespace or EOF, "+"but got '%s' "+
			"instead.", r)
	} else if r == eof {
		return lx.stop()
	}
	return lx.pop()
}

// lexWhiteSpace consumes one or more whitespace characters, otherwise the
// lexer fails.
func lexWhiteSpace(lx *lexer, nextState stateFn) stateFn {
	r := lx.next()
	if !isWhiteSpace(r) {
		return lx.errf("Expected white space, but got '%s' instead.", r)
	}
	lx.push(nextState)
	return lexWhiteSpaceContinue
}

// lexWhiteSpaceContinue consumes zero or more whitespace characters. It also
// looks for comments and consumes those as well.
func lexWhiteSpaceContinue(lx *lexer) stateFn {
	r := lx.next()
	switch {
	case r == commentStart:
		lx.ignore()
		return lexComment
	case !isWhiteSpace(r):
		lx.backup()
		lx.ignore()
		return lx.pop()
	}
	lx.ignore()
	return lexWhiteSpaceContinue
}

// lexComment consumes any sequence of characters up to a <eol> or EOF.
func lexComment(lx *lexer) stateFn {
	r := lx.peek()
	if isNL(r) || r == eof {
		lx.emit(itemComment)
		return lexWhiteSpaceContinue
	}
	lx.accept(r)
	return lexComment
}

func isOrdinaryChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') || r == '!' || r == '%' || r == '&' ||
		r == '(' || r == ')' || r == '*' || r == '+' || r == ',' ||
		r == '-' || r == '.' || r == '/' || r == ':' || r == '<' ||
		r == '=' || r == '>' || r == '?' || r == '@' || r == '\\' ||
		r == '^' || r == '`' || r == '{' || r == '|' || r == '}' || r == '~'
}

func isNonBlankChar(r rune) bool {
	return isOrdinaryChar(r) || r == '"' || r == '#' || r == '$' ||
		r == '\'' || r == '_' || r == ';' || r == '[' || r == ']'
}

func isTextLeadChar(r rune) bool {
	return isOrdinaryChar(r) || r == '"' || r == '#' || r == '$' ||
		r == '\'' || r == '_' || r == ' ' || r == '\t' || r == '[' || r == ']'
}

func isPrintChar(r rune) bool {
	return isTextLeadChar(r) || r == ';'
}

func isWhiteSpace(r rune) bool {
	return r == ' ' || r == '\t' || isNL(r)
}

func isNL(r rune) bool {
	return r == '\n' || r == '\r'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (itype itemType) String() string {
	switch itype {
	case itemError:
		return "Error"
	case itemEOF:
		return "EOF"
	case itemVersion:
		return "Version"
	case itemComment:
		return "Comment"
	case itemGlobal:
		return "Global"
	case itemStop:
		return "Stop"
	case itemDataBlockStart:
		return "DataBlockStart"
	case itemSaveFrameStart:
		return "SaveFrameStart"
	case itemSaveFrameEnd:
		return "SaveFrameEnd"
	case itemLoop:
		return "Loop"
	case itemDataTag:
		return "DataTag"
	case itemDataOmitted:
		return "DataOmitted"
	case itemDataMissing:
		return "DataMissing"
	case itemDataInteger:
		return "DataInteger"
	case itemDataFloat:
		return "DataFloat"
	case itemDataString:
		return "DataString"
	}
	panic(sf("BUG: Unknown type '%s'.", itype))
}
