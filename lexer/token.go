package lexer

import "github.com/kamichidu/go-javaparser/syntax"

// TokenType defines the category of a scanned token.
type TokenType int

const (
	TokEOF TokenType = iota
	TokError
	TokIdentifier // e.g. "MyClass", "varName"
	TokKeyword    // e.g. "public", "class", "void"
	TokLiteral    // e.g. "hello", 123, true, null
	TokSymbol     // e.g. "{", "}", "(", ")", ";", ",", ".", "@", "<", ">"
	TokComment    // e.g. "// comment", "/* block comment */", "/** Javadoc */"
)

func (t TokenType) String() string {
	switch t {
	case TokEOF:
		return "EOF"
	case TokError:
		return "Error"
	case TokIdentifier:
		return "Identifier"
	case TokKeyword:
		return "Keyword"
	case TokLiteral:
		return "Literal"
	case TokSymbol:
		return "Symbol"
	case TokComment:
		return "Comment"
	default:
		return "Unknown"
	}
}

// Token represents a single lexical token scanned from the source.
type Token struct {
	Type    TokenType
	Literal string
	Range   syntax.Range
}
