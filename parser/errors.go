package parser

import (
	"fmt"

	"github.com/kamichidu/go-javaparser/syntax"
)

// ParseError represents a compilation or parsing syntax error.
type ParseError struct {
	Message string
	Range   syntax.Range
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("Parse error at %d:%d: %s", e.Range.Start.Line, e.Range.Start.Col, e.Message)
}
