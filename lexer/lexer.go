package lexer

import (
	"unicode"

	"github.com/kamichidu/go-javaparser/syntax"
)

// Lexer scans a Java source string into tokens.
type Lexer struct {
	input    []rune
	position int  // current position in input (index of ch)
	readPos  int  // current reading position in input (next char index)
	ch       rune // current char under examination
	line     int  // 1-based line
	col      int  // 1-based column
}

// New creates a Lexer from Java source code.
func New(source string) *Lexer {
	l := &Lexer{
		input:    []rune(source),
		line:     1,
		col:      0,
		readPos:  0,
		position: 0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}

	l.position = l.readPos
	l.readPos++

	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// NextToken returns the next scanned Token.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.ch == 0 {
		return Token{
			Type:    TokEOF,
			Literal: "",
			Range: syntax.Range{
				Start: syntax.Position{Line: l.line, Col: l.col},
				End:   syntax.Position{Line: l.line, Col: l.col},
			},
		}
	}

	startPos := syntax.Position{Line: l.line, Col: l.col}

	// Handle Comments or division operator '/'
	if l.ch == '/' {
		if l.peekChar() == '/' {
			return l.scanSingleLineComment(startPos)
		} else if l.peekChar() == '*' {
			return l.scanBlockComment(startPos)
		}
	}

	// Handle String literals
	if l.ch == '"' {
		if l.peekChar() == '"' {
			// Check for text block (Java 15+ triple quotes """ )
			// If we peek the next next, or read ahead, let's see.
			if l.readPos+1 < len(l.input) && l.input[l.readPos+1] == '"' {
				return l.scanTextBlockLiteral(startPos)
			}
		}
		return l.scanStringLiteral(startPos)
	}

	// Handle Character literals
	if l.ch == '\'' {
		return l.scanCharLiteral(startPos)
	}

	// Handle Symbols
	if isSymbol(l.ch) {
		ch := l.ch
		lit := string(ch)
		l.readChar()

		// Handle composite symbols if necessary (e.g., "->", "::", "@interface")
		// But in a simple LL(k) declaration parser, we can just treat them as separate or individual symbols,
		// or combine common ones like "->", "::".
		if ch == '-' && l.ch == '>' {
			lit = "->"
			l.readChar()
		} else if ch == ':' && l.ch == ':' {
			lit = "::"
			l.readChar()
		}

		endPos := syntax.Position{Line: l.line, Col: l.col}
		return Token{
			Type:    TokSymbol,
			Literal: lit,
			Range:   syntax.Range{Start: startPos, End: endPos},
		}
	}

	// Handle Identifiers & Keywords
	if isIdentifierStart(l.ch) {
		literal := l.readIdentifier()
		endPos := syntax.Position{Line: l.line, Col: l.col}
		tokType := TokIdentifier
		if isKeyword(literal) {
			tokType = TokKeyword
		} else if literal == "true" || literal == "false" || literal == "null" {
			tokType = TokLiteral
		}

		return Token{
			Type:    tokType,
			Literal: literal,
			Range:   syntax.Range{Start: startPos, End: endPos},
		}
	}

	// Handle Numbers
	if unicode.IsDigit(l.ch) {
		literal := l.readNumber()
		endPos := syntax.Position{Line: l.line, Col: l.col}
		return Token{
			Type:    TokLiteral,
			Literal: literal,
			Range:   syntax.Range{Start: startPos, End: endPos},
		}
	}

	// Error token for unknown characters
	ch := l.ch
	l.readChar()
	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokError,
		Literal: string(ch),
		Range:   syntax.Range{Start: startPos, End: endPos},
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) scanSingleLineComment(start syntax.Position) Token {
	l.readChar() // consume first '/'
	l.readChar() // consume second '/'

	startIdx := l.position
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.readChar()
	}
	lit := "//" + string(l.input[startIdx:l.position])

	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokComment,
		Literal: lit,
		Range:   syntax.Range{Start: start, End: endPos},
	}
}

func (l *Lexer) scanBlockComment(start syntax.Position) Token {
	l.readChar() // consume '/'
	l.readChar() // consume '*'

	startIdx := l.position
	for {
		if l.ch == 0 {
			break
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			break
		}
		l.readChar()
	}
	lit := "/*" + string(l.input[startIdx:l.position])

	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokComment,
		Literal: lit,
		Range:   syntax.Range{Start: start, End: endPos},
	}
}

func (l *Lexer) scanStringLiteral(start syntax.Position) Token {
	l.readChar() // consume starting '"'
	startIdx := l.position

	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // consume escape prefix
		}
		l.readChar()
	}

	lit := string(l.input[startIdx:l.position])
	if l.ch == '"' {
		l.readChar() // consume ending '"'
	}

	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokLiteral,
		Literal: `"` + lit + `"`,
		Range:   syntax.Range{Start: start, End: endPos},
	}
}

func (l *Lexer) scanTextBlockLiteral(start syntax.Position) Token {
	l.readChar() // consume first '"'
	l.readChar() // consume second '"'
	l.readChar() // consume third '"'

	startIdx := l.position

	for {
		if l.ch == 0 {
			break
		}
		if l.ch == '"' && l.peekChar() == '"' {
			// check for third quote
			if l.readPos+1 < len(l.input) && l.input[l.readPos+1] == '"' {
				l.readChar() // consume first '"'
				l.readChar() // consume second '"'
				l.readChar() // consume third '"'
				break
			}
		}
		if l.ch == '\\' {
			l.readChar()
		}
		l.readChar()
	}

	lit := string(l.input[startIdx : l.position-3]) // exclude ending triple quotes
	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokLiteral,
		Literal: `"""` + lit + `"""`,
		Range:   syntax.Range{Start: start, End: endPos},
	}
}

func (l *Lexer) scanCharLiteral(start syntax.Position) Token {
	l.readChar() // consume starting '\''
	startIdx := l.position

	for l.ch != '\'' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
		}
		l.readChar()
	}

	lit := string(l.input[startIdx:l.position])
	if l.ch == '\'' {
		l.readChar() // consume ending '\''
	}

	endPos := syntax.Position{Line: l.line, Col: l.col}
	return Token{
		Type:    TokLiteral,
		Literal: `'` + lit + `'`,
		Range:   syntax.Range{Start: start, End: endPos},
	}
}

func (l *Lexer) readIdentifier() string {
	startIdx := l.position
	for isIdentifierPart(l.ch) {
		l.readChar()
	}
	return string(l.input[startIdx:l.position])
}

func (l *Lexer) readNumber() string {
	startIdx := l.position
	// Simple number scanning. Covers decimals, hex (0x...), floats (1.23, 1f, etc.)
	for isNumberChar(l.ch) {
		l.readChar()
	}
	return string(l.input[startIdx:l.position])
}

func isSymbol(ch rune) bool {
	symbols := []rune{'{', '}', '(', ')', '[', ']', ';', ',', '.', '@', '<', '>', '&', '=', '*', ':', '-', '!'}
	for _, s := range symbols {
		if s == ch {
			return true
		}
	}
	return false
}

func isIdentifierStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_' || ch == '$'
}

func isIdentifierPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '$'
}

func isNumberChar(ch rune) bool {
	return unicode.IsDigit(ch) || ch == '.' || ch == 'x' || ch == 'X' ||
		ch == 'a' || ch == 'b' || ch == 'c' || ch == 'd' || ch == 'e' || ch == 'f' ||
		ch == 'A' || ch == 'B' || ch == 'C' || ch == 'D' || ch == 'E' || ch == 'F' ||
		ch == 'l' || ch == 'L' ||
		ch == 'p' || ch == 'P' || ch == '+' || ch == '-'
}

var javaKeywords = map[string]bool{
	"abstract":     true,
	"assert":       true,
	"boolean":      true,
	"break":        true,
	"byte":         true,
	"case":         true,
	"catch":        true,
	"char":         true,
	"class":        true,
	"const":        true,
	"continue":     true,
	"default":      true,
	"do":           true,
	"double":       true,
	"else":         true,
	"enum":         true,
	"extends":      true,
	"final":        true,
	"finally":      true,
	"float":        true,
	"for":          true,
	"goto":         true,
	"if":           true,
	"implements":   true,
	"import":       true,
	"instanceof":   true,
	"int":          true,
	"interface":    true,
	"long":         true,
	"native":       true,
	"new":          true,
	"package":      true,
	"private":      true,
	"protected":    true,
	"public":       true,
	"return":       true,
	"short":        true,
	"static":       true,
	"strictfp":     true,
	"super":        true,
	"switch":       true,
	"synchronized": true,
	"this":         true,
	"throw":        true,
	"throws":       true,
	"transient":    true,
	"try":          true,
	"void":         true,
	"volatile":     true,
	"while":        true,
	"record":       true, // Contextual keyword
	"sealed":       true,
	"non-sealed":   true,
	"permits":      true,
	"yield":        true,
}

func isKeyword(s string) bool {
	return javaKeywords[s]
}
