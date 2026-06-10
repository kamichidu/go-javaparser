package completion

import (
	"strings"
	"unicode"

	"github.com/kamichidu/go-javaparser/ast"
	"github.com/kamichidu/go-javaparser/lexer"
	"github.com/kamichidu/go-javaparser/parser"
	"github.com/kamichidu/go-javaparser/syntax"
)

type ContextKind string

const (
	ContextUnknown      ContextKind = "unknown"
	ContextTypeName     ContextKind = "type_name"
	ContextMemberAccess ContextKind = "member_access"
	ContextImport       ContextKind = "import"
	ContextAnnotation   ContextKind = "annotation"
)

type CompletionContext struct {
	SourceID        string
	Position        syntax.Position
	Kind            ContextKind
	Receiver        *ReceiverExpr
	MemberPrefix    string // Prefix of member name being typed (e.g. "f" in "System.out.f")
	EnclosingType   string
	EnclosingMethod string
	PackageName     string
	Imports         []ImportFact
	Locals          []LocalFact
}

type ReceiverExprKind string

const (
	ReceiverUnknown          ReceiverExprKind = "unknown"
	ReceiverSimpleName       ReceiverExprKind = "simple_name"
	ReceiverQualifiedName    ReceiverExprKind = "qualified_name"
	ReceiverMethodInvocation ReceiverExprKind = "method_invocation"
	ReceiverNewExpression    ReceiverExprKind = "new_expression"
	ReceiverParenthesized    ReceiverExprKind = "parenthesized"
)

type ReceiverExpr struct {
	RawText string
	Range   syntax.Range
	Kind    ReceiverExprKind
}

type ImportFact struct {
	Name     string
	IsStatic bool
	IsStar   bool
	Range    syntax.Range
}

type LocalFact struct {
	Name  string
	Type  string
	Range syntax.Range
}

// ExtractContext parses the file and extracts the completion context.
func ExtractContext(content string, sourceID string, pos syntax.Position) CompletionContext {
	ctx := CompletionContext{
		SourceID: sourceID,
		Position: pos,
		Kind:     ContextUnknown,
	}

	// 1. Lex and Parse file to gather AST declarations
	l := lexer.New(content)
	p := parser.New(l, sourceID, parser.WithPolicy(2)) // SubscribeOccurrences (value 2)

	file, err := p.ParseFile()
	if err == nil && file != nil {
		if file.Package != nil {
			ctx.PackageName = file.Package.Name
		}
		for _, imp := range file.Imports {
			ctx.Imports = append(ctx.Imports, ImportFact{
				Name:     imp.Name,
				IsStatic: imp.IsStatic,
				IsStar:   imp.IsStar,
				Range:    imp.Range,
			})
		}

		// Find enclosing type, enclosing method, and local declarations
		findEnclosingDeclarations(file, pos, &ctx)
	}

	// 2. Identify syntax context and extract receiver expression from the content
	analyzeSyntaxContext(content, pos, &ctx)

	return ctx
}

func findEnclosingDeclarations(file *ast.File, pos syntax.Position, ctx *CompletionContext) {
	for _, tDecl := range file.Types {
		if contains(tDecl.Range, pos) {
			ctx.EnclosingType = tDecl.Name
			// Walk type members to find method or constructor enclosing pos
			for _, member := range tDecl.Members {
				switch m := member.(type) {
				case *ast.MethodDecl:
					if contains(m.Range, pos) {
						ctx.EnclosingMethod = m.Name
						// Add parameters declared inside the method
						for _, param := range m.Parameters {
							ctx.Locals = append(ctx.Locals, LocalFact{
								Name:  param.Name,
								Type:  param.Type,
								Range: param.Range,
							})
						}
						// Walk the block body for local declarations prior to pos
						if m.Body != nil {
							walkBlockLocals(m.Body, pos, ctx)
						}
					}
				case *ast.ConstructorDecl:
					if contains(m.Range, pos) {
						ctx.EnclosingMethod = m.Name
						for _, param := range m.Parameters {
							ctx.Locals = append(ctx.Locals, LocalFact{
								Name:  param.Name,
								Type:  param.Type,
								Range: param.Range,
							})
						}
						if m.Body != nil {
							walkBlockLocals(m.Body, pos, ctx)
						}
					}
				}
			}
			break
		}
	}
}

func walkBlockLocals(block *ast.Block, pos syntax.Position, ctx *CompletionContext) {
	if block == nil {
		return
	}
	for _, localNode := range block.Locals {
		switch node := localNode.(type) {
		case *ast.LocalVarDecl:
			// Variable must be declared strictly before the cursor position
			if isBefore(node.Range.Start, pos) {
				ctx.Locals = append(ctx.Locals, LocalFact{
					Name:  node.Name,
					Type:  node.Type,
					Range: node.Range,
				})
			}
		case *ast.Block:
			if contains(node.Range, pos) {
				walkBlockLocals(node, pos, ctx)
			}
		}
	}
}

func contains(r syntax.Range, pos syntax.Position) bool {
	if pos.Line < r.Start.Line || pos.Line > r.End.Line {
		return false
	}
	if pos.Line == r.Start.Line && pos.Col < r.Start.Col {
		return false
	}
	if pos.Line == r.End.Line && pos.Col > r.End.Col {
		return false
	}
	return true
}

func isBefore(p1, p2 syntax.Position) bool {
	if p1.Line < p2.Line {
		return true
	}
	if p1.Line == p2.Line && p1.Col < p2.Col {
		return true
	}
	return false
}

func analyzeSyntaxContext(content string, pos syntax.Position, ctx *CompletionContext) {
	lines := strings.Split(content, "\n")
	lineIdx := pos.Line - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return
	}

	line := lines[lineIdx]
	runes := []rune(line)
	colIdx := pos.Col - 1 // 0-based col index
	if colIdx < 0 || colIdx > len(runes) {
		colIdx = len(runes)
	}

	prefixRunes := runes[:colIdx]
	prefixText := string(prefixRunes)

	// Check if this is an import context
	trimmedLine := strings.TrimSpace(prefixText)
	if strings.HasPrefix(trimmedLine, "import ") {
		ctx.Kind = ContextImport
		return
	}

	// Check if this is an annotation context
	if strings.HasPrefix(trimmedLine, "@") || strings.Contains(trimmedLine, " @") {
		ctx.Kind = ContextAnnotation
		return
	}

	// Scan backwards from the end of prefixText to find the last unnested dot "."
	text := strings.TrimRight(prefixText, " \t")
	if text == "" {
		return
	}

	lastDotIdx := -1
	parenDepth := 0
	braceDepth := 0
	angleDepth := 0
	textRunes := []rune(text)

	for i := len(textRunes) - 1; i >= 0; i-- {
		r := textRunes[i]
		if r == ')' {
			parenDepth++
		} else if r == '(' {
			parenDepth--
		} else if r == '}' {
			braceDepth++
		} else if r == '{' {
			braceDepth--
		} else if r == '>' {
			angleDepth++
		} else if r == '<' {
			angleDepth--
		}

		if parenDepth == 0 && braceDepth == 0 && angleDepth == 0 {
			if r == '.' {
				lastDotIdx = i
				break
			}
			// If we hit other expression delimiters, stop scanning
			if r == ';' || r == '=' || r == ',' || r == '{' || r == '}' {
				break
			}
		}
	}

	if lastDotIdx != -1 {
		ctx.Kind = ContextMemberAccess
		ctx.MemberPrefix = string(textRunes[lastDotIdx+1:])

		// Backtrack from lastDotIdx to find the actual start of receiver expression
		receiverCleanText, startCol := backtrackReceiver(textRunes[:lastDotIdx+1]) // include the dot so backtrack skips it
		if receiverCleanText != "" {
			ctx.Receiver = &ReceiverExpr{
				RawText: receiverCleanText,
				Range: syntax.Range{
					Start: syntax.Position{Line: pos.Line, Col: startCol + 1},
					End:   syntax.Position{Line: pos.Line, Col: lastDotIdx + 1},
				},
				Kind: classifyReceiverKind(receiverCleanText),
			}
		}
		return
	}

	// Default or type name context
	ctx.Kind = ContextTypeName
}

func backtrackReceiver(runes []rune) (string, int) {
	n := len(runes)
	if n == 0 {
		return "", 0
	}

	// Skip the trailing dot "."
	endIdx := n - 1
	if runes[endIdx] == '.' {
		endIdx--
	}

	// Skip whitespace before dot
	for endIdx >= 0 && (runes[endIdx] == ' ' || runes[endIdx] == '\t') {
		endIdx--
	}

	if endIdx < 0 {
		return "", 0
	}

	parenDepth := 0
	angleDepth := 0
	braceDepth := 0
	startIdx := endIdx

	for startIdx >= 0 {
		r := runes[startIdx]

		if r == ')' {
			parenDepth++
			startIdx--
			continue
		} else if r == '(' {
			parenDepth--
			startIdx--
			continue
		}

		if r == '>' {
			angleDepth++
			startIdx--
			continue
		} else if r == '<' {
			angleDepth--
			startIdx--
			continue
		}

		if r == '}' {
			braceDepth++
			startIdx--
			continue
		} else if r == '{' {
			braceDepth--
			startIdx--
			continue
		}

		// If we are balanced, we can stop on expression delimiters
		if parenDepth == 0 && angleDepth == 0 && braceDepth == 0 {
			// Check if we hit a statement delimiter or assignment symbol
			if r == ';' || r == '=' || r == ',' || r == '{' || r == '}' {
				break
			}
			// Check if we hit whitespace.
			if r == ' ' || r == '\t' {
				// We peek ahead to see if the word we are bypassing is "new"
				if startIdx >= 3 && string(runes[startIdx-3:startIdx]) == "new" {
					startIdx -= 3
					continue
				}
				break
			}
		}

		startIdx--
	}

	startIdx++ // shift to the actual start of expression
	if startIdx > endIdx {
		return "", 0
	}

	expr := strings.TrimSpace(string(runes[startIdx : endIdx+1]))
	return expr, startIdx
}

func classifyReceiverKind(text string) ReceiverExprKind {
	if strings.HasPrefix(text, "new ") {
		return ReceiverNewExpression
	}
	if strings.HasSuffix(text, ")") {
		if strings.HasPrefix(text, "(") && !strings.Contains(text, ".") {
			return ReceiverParenthesized
		}
		return ReceiverMethodInvocation
	}
	if strings.Contains(text, ".") {
		return ReceiverQualifiedName
	}
	if isWord(text) {
		return ReceiverSimpleName
	}
	return ReceiverUnknown
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' {
			return false
		}
	}
	return true
}
