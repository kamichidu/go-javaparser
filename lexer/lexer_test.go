package lexer

import (
	"testing"
)

func TestLexerBasic(t *testing.T) {
	input := `package com.example;
import java.util.List;

/**
 * Javadoc comment
 */
public class Main {
    // line comment
    private String name = "hello";
    private String block = """
        text block
        """;
    private char letter = 'a';

    public void run(int count) {
        /* block comment */
    }
}`

	l := New(input)

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
		expectedLine    int
	}{
		{TokKeyword, "package", 1},
		{TokIdentifier, "com", 1},
		{TokSymbol, ".", 1},
		{TokIdentifier, "example", 1},
		{TokSymbol, ";", 1},

		{TokKeyword, "import", 2},
		{TokIdentifier, "java", 2},
		{TokSymbol, ".", 2},
		{TokIdentifier, "util", 2},
		{TokSymbol, ".", 2},
		{TokIdentifier, "List", 2},
		{TokSymbol, ";", 2},

		{TokComment, "/**\n * Javadoc comment\n */", 4},

		{TokKeyword, "public", 7},
		{TokKeyword, "class", 7},
		{TokIdentifier, "Main", 7},
		{TokSymbol, "{", 7},

		{TokComment, "// line comment", 8},

		{TokKeyword, "private", 9},
		{TokIdentifier, "String", 9},
		{TokIdentifier, "name", 9},
		{TokSymbol, "=", 9},
		{TokLiteral, `"hello"`, 9},
		{TokSymbol, ";", 9},

		{TokKeyword, "private", 10},
		{TokIdentifier, "String", 10},
		{TokIdentifier, "block", 10},
		{TokSymbol, "=", 10},
		{TokLiteral, `"""
        text block
        """`, 10},
		{TokSymbol, ";", 12},

		{TokKeyword, "private", 13},
		{TokKeyword, "char", 13},
		{TokIdentifier, "letter", 13},
		{TokSymbol, "=", 13},
		{TokLiteral, `'a'`, 13},
		{TokSymbol, ";", 13},

		{TokKeyword, "public", 15},
		{TokKeyword, "void", 15},
		{TokIdentifier, "run", 15},
		{TokSymbol, "(", 15},
		{TokKeyword, "int", 15},
		{TokIdentifier, "count", 15},
		{TokSymbol, ")", 15},
		{TokSymbol, "{", 15},

		{TokComment, "/* block comment */", 16},

		{TokSymbol, "}", 17},
		{TokSymbol, "}", 18},
		{TokEOF, "", 18},
	}

	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("[%d] type mismatch: expected=%s, got=%s (literal=%q)", i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("[%d] literal mismatch: expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
		if tok.Range.Start.Line != tt.expectedLine {
			t.Fatalf("[%d] line mismatch: expected=%d, got=%d (literal=%q)", i, tt.expectedLine, tok.Range.Start.Line, tok.Literal)
		}
	}
}
