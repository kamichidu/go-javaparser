package parser

import (
	"github.com/kamichidu/go-javaparser/ast"
	"github.com/kamichidu/go-javaparser/event"
	"github.com/kamichidu/go-javaparser/lexer"
	"github.com/kamichidu/go-javaparser/syntax"
)

// Option defines a functional configuration for Parser Core.
type Option func(*Parser)

// WithEventSink registers an EventSink for Event Projection.
func WithEventSink(sink event.EventSink) Option {
	return func(p *Parser) {
		p.sink = sink
	}
}

// WithPolicy registers the subscription policy.
func WithPolicy(policy event.SubscriptionPolicy) Option {
	return func(p *Parser) {
		p.policy = policy
	}
}

// WithSignatureOnly enables/disables the On-Demand Body Skipping.
func WithSignatureOnly(sigOnly bool) Option {
	return func(p *Parser) {
		p.signatureOnly = sigOnly
	}
}

// Parser is the handwritten LL(k) Parser Core.
type Parser struct {
	l             *lexer.Lexer
	curTok        lexer.Token
	peekTok       lexer.Token
	sourceID      string
	sink          event.EventSink
	policy        event.SubscriptionPolicy
	signatureOnly bool
	errors        []error
	hasError      bool
}

// New creates a Parser Core instance.
func New(l *lexer.Lexer, sourceID string, opts ...Option) *Parser {
	p := &Parser{
		l:        l,
		sourceID: sourceID,
		policy:   event.DefaultPolicy,
	}

	for _, opt := range opts {
		opt(p)
	}

	// Read first two tokens to seed curTok and peekTok (LL(2) lookahead)
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curTok = p.peekTok
	p.peekTok = p.l.NextToken()
	// Skip comments but optionally capture Javadoc if needed in the future.
	// For now, we skip comments to keep parser recursive-descent clean.
	for p.curTok.Type == lexer.TokComment {
		p.curTok = p.peekTok
		p.peekTok = p.l.NextToken()
	}
}

func (p *Parser) emit(evt event.SourceEvent) {
	if p.sink != nil && !p.hasError {
		err := p.sink.Emit(evt)
		if err != nil {
			p.reportError("Failed to emit event: "+err.Error(), evt.GetRange())
		}
	}
}

func (p *Parser) reportError(msg string, r syntax.Range) {
	p.errors = append(p.errors, &ParseError{Message: msg, Range: r})
	if !p.hasError {
		p.hasError = true
		// Emit ParseErrorEvent. Under v1 specification, we immediately stop further emission.
		p.emit(event.ParseErrorEvent{
			SourceID: p.sourceID,
			Message:  msg,
			Range:    r,
		})
	}
}

// ParseFile parses the Java source and returns the AST (AST Projection)
// and/or streams the occurrences (Event Projection).
func (p *Parser) ParseFile() (*ast.File, error) {
	fileRange := syntax.Range{
		Start: p.curTok.Range.Start,
	}

	p.emit(event.StartFileEvent{SourceID: p.sourceID})

	file := &ast.File{
		Path: p.sourceID,
	}

	// 1. Package Declaration
	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "package" {
		pkg := p.parsePackage()
		file.Package = pkg
		p.emit(event.PackageDeclEvent{
			Name:  pkg.Name,
			Range: pkg.Range,
		})
	}

	// 2. Import Declarations
	for p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "import" {
		imp := p.parseImport()
		file.Imports = append(file.Imports, imp)
		p.emit(event.ImportDeclEvent{
			Name:     imp.Name,
			IsStatic: imp.IsStatic,
			IsStar:   imp.IsStar,
			Range:    imp.Range,
		})
	}

	// 3. Type Declarations
	for p.curTok.Type != lexer.TokEOF && !p.hasError {
		typeDecl := p.parseTypeDeclaration()
		if typeDecl != nil {
			file.Types = append(file.Types, typeDecl)
		} else {
			// If we can't parse a type declaration, try to skip a token to recover,
			// or break if we are stuck at EOF to prevent infinite loops.
			if p.curTok.Type == lexer.TokEOF {
				break
			}
			p.nextToken()
		}
	}

	fileRange.End = p.curTok.Range.Start
	file.Range = fileRange

	if p.hasError {
		return nil, p.errors[0]
	}

	p.emit(event.EndFileEvent{SourceID: p.sourceID})
	return file, nil
}

// ParseLocalScope parses a specific given range/text.
// Implements "Consumer-Driven Range Parsing" where the consumer decides the scope.
func (p *Parser) ParseLocalScope(mergeScope string) (*ast.Block, error) {
	scopeRange := syntax.Range{
		Start: p.curTok.Range.Start,
	}

	p.emit(event.StartDeltaScopeEvent{
		SourceID:   p.sourceID,
		MergeScope: mergeScope,
		Range:      scopeRange,
	})

	block := p.parseBlock()

	scopeRange.End = p.curTok.Range.Start
	block.Range = scopeRange

	if p.hasError {
		return nil, p.errors[0] // Reports as "Partial Parse Failure"
	}

	p.emit(event.EndDeltaScopeEvent{
		SourceID:   p.sourceID,
		MergeScope: mergeScope,
		Range:      scopeRange,
	})

	return block, nil
}

func (p *Parser) parsePackage() *ast.Package {
	start := p.curTok.Range.Start
	p.nextToken() // consume "package"

	name := p.parseQualifiedName()

	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == ";" {
		p.nextToken() // consume ";"
	} else {
		p.reportError("Expected ';' after package declaration", p.curTok.Range)
	}

	return &ast.Package{
		Name:  name,
		Range: syntax.Range{Start: start, End: p.curTok.Range.Start},
	}
}

func (p *Parser) parseImport() *ast.Import {
	start := p.curTok.Range.Start
	p.nextToken() // consume "import"

	isStatic := false
	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "static" {
		isStatic = true
		p.nextToken()
	}

	name := p.parseQualifiedName()
	isStar := false

	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "." {
		p.nextToken() // consume "."
		if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "*" {
			isStar = true
			p.nextToken() // consume "*"
		} else {
			p.reportError("Expected '*' after '.' in import wildcard", p.curTok.Range)
		}
	}

	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == ";" {
		p.nextToken() // consume ";"
	} else {
		p.reportError("Expected ';' after import declaration", p.curTok.Range)
	}

	return &ast.Import{
		Name:     name,
		IsStatic: isStatic,
		IsStar:   isStar,
		Range:    syntax.Range{Start: start, End: p.curTok.Range.Start},
	}
}

func (p *Parser) parseTypeDeclaration() *ast.TypeDecl {
	annotations := p.parseAnnotations()
	modifiers := p.parseModifiers()

	// Check if this is an annotation definition "@interface"
	var kind syntax.TypeKind
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "@" && p.peekTok.Literal == "interface" {
		p.nextToken() // consume "@"
		p.nextToken() // consume "interface"
		kind = syntax.KindAnnotationType
	} else if p.curTok.Type == lexer.TokKeyword {
		switch p.curTok.Literal {
		case "class":
			kind = syntax.KindClass
			p.nextToken()
		case "interface":
			kind = syntax.KindInterface
			p.nextToken()
		case "enum":
			kind = syntax.KindEnum
			p.nextToken()
		case "record":
			kind = syntax.KindRecord
			p.nextToken()
		default:
			// Recover or skip
			return nil
		}
	} else {
		return nil
	}

	if p.curTok.Type != lexer.TokIdentifier {
		p.reportError("Expected type name after type kind", p.curTok.Range)
		return nil
	}

	name := p.curTok.Literal
	startPos := p.curTok.Range.Start
	p.nextToken() // consume name

	// Parse optional Generics
	var generics []*ast.TypeParam
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "<" {
		generics = p.parseGenerics()
	}

	// Extends & Implements
	var extends []string
	var implements []string

	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "extends" {
		p.nextToken()
		extends = p.parseTypeList()
	}

	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "implements" {
		p.nextToken()
		implements = p.parseTypeList()
	}

	typeDecl := &ast.TypeDecl{
		Kind:       kind,
		Name:       name,
		Modifiers:  modifiers,
		Extends:    extends,
		Implements: implements,
		Generics:   generics,
		Range:      syntax.Range{Start: startPos},
	}

	// Convert ast annotations list to string slice for event
	var annotationNames []string
	for _, ann := range annotations {
		annotationNames = append(annotationNames, ann.Name)
		typeDecl.Annotations = append(typeDecl.Annotations, ann)
	}

	p.emit(event.TypeDeclEvent{
		Kind:        kind,
		Name:        name,
		Modifiers:   modifiers,
		Annotations: annotationNames,
		Range:       typeDecl.Range,
	})

	// Parse Type Body
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "{" {
		p.nextToken() // consume "{"
		for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != "}" {
			if p.curTok.Type == lexer.TokEOF || p.hasError {
				break
			}
			member := p.parseTypeMember(name)
			if member != nil {
				typeDecl.Members = append(typeDecl.Members, member)
			} else {
				p.synchronizeMember()
			}
		}
		if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "}" {
			p.nextToken() // consume "}"
		} else {
			p.reportError("Expected '}' at the end of class definition", p.curTok.Range)
		}
	} else {
		p.reportError("Expected '{' at start of class body", p.curTok.Range)
	}

	typeDecl.Range.End = p.curTok.Range.Start
	return typeDecl
}

func (p *Parser) parseTypeMember(className string) ast.MemberNode {
	annotations := p.parseAnnotations()
	modifiers := p.parseModifiers()

	// Check for nested types
	if p.curTok.Type == lexer.TokKeyword && (p.curTok.Literal == "class" || p.curTok.Literal == "interface" || p.curTok.Literal == "enum" || p.curTok.Literal == "record") {
		// Put back modifiers and annotations mentally, nested parsing will capture them if we design properly.
		// For simplicity, we parse nested classes here:
		p.nextToken() // consume keyword
		// Just parse it as a nested type
		p.reportError("Nested types are not fully supported inside member declarations in v1", p.curTok.Range)
		return nil
	}

	// Check if it is a constructor
	if p.curTok.Type == lexer.TokIdentifier && p.curTok.Literal == className && p.peekTok.Literal == "(" {
		return p.parseConstructor(modifiers, annotations)
	}

	// Must be field or method. First read Type.
	memberType := p.parseType()

	if p.curTok.Type != lexer.TokIdentifier {
		p.reportError("Expected identifier for field or method", p.curTok.Range)
		return nil
	}

	memberName := p.curTok.Literal
	memberRange := p.curTok.Range
	p.nextToken() // consume member name

	// If followed by '(', it is a Method
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "(" {
		return p.parseMethod(modifiers, annotations, memberType, memberName)
	}

	// Otherwise, it is a Field
	return p.parseField(modifiers, annotations, memberType, memberName, memberRange)
}

func (p *Parser) parseField(modifiers []string, annotations []*ast.Annotation, memberType string, name string, memberRange syntax.Range) *ast.FieldDecl {
	// Skip potential initializer
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "=" {
		p.nextToken() // consume "="
		// Skip initializer expression up to semicolon
		for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != ";" {
			if p.curTok.Type == lexer.TokEOF {
				break
			}
			p.nextToken()
		}
	}

	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == ";" {
		p.nextToken() // consume ";"
	} else {
		p.reportError("Expected ';' after field declaration", p.curTok.Range)
	}

	field := &ast.FieldDecl{
		Name:        name,
		Type:        memberType,
		Modifiers:   modifiers,
		Annotations: annotations,
		Range:       memberRange,
	}

	p.emit(event.FieldDeclEvent{
		Name:        name,
		DataType:    memberType,
		Modifiers:   modifiers,
		IsEnumConst: false,
		Range:       field.Range,
	})

	return field
}

func (p *Parser) parseMethod(modifiers []string, annotations []*ast.Annotation, returnType string, name string) *ast.MethodDecl {
	p.nextToken() // consume "("
	params := p.parseParameters()
	p.nextToken() // consume ")"

	var exceptions []string
	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "throws" {
		p.nextToken()
		exceptions = p.parseTypeList()
	}

	method := &ast.MethodDecl{
		Name:        name,
		ReturnType:  returnType,
		Modifiers:   modifiers,
		Annotations: annotations,
		Parameters:  params,
		Exceptions:  exceptions,
		Range:       syntax.Range{Start: p.curTok.Range.Start},
	}

	p.emit(event.MethodDeclEvent{
		Name:       name,
		ReturnType: returnType,
		Modifiers:  modifiers,
		Parameters: params,
		Range:      method.Range,
	})

	// Method body block parsing or abstract semicolon
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == ";" {
		p.nextToken() // consume ";"
	} else if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "{" {
		// Enforce "Consumer-Driven Projections":
		// If signatureOnly is true, OR SubscribeOccurrences is disabled,
		// then the method body conceptually "does not exist". We skip it completely!
		if p.signatureOnly || (p.policy&event.SubscribeOccurrences) == 0 {
			p.skipBalanced('{', '}')
		} else {
			method.Body = p.parseBlock()
		}
	} else {
		p.reportError("Expected '{' or ';' after method signature", p.curTok.Range)
	}

	method.Range.End = p.curTok.Range.Start
	return method
}

func (p *Parser) parseConstructor(modifiers []string, annotations []*ast.Annotation) *ast.ConstructorDecl {
	name := p.curTok.Literal
	p.nextToken() // consume className identifier
	p.nextToken() // consume "("

	params := p.parseParameters()
	p.nextToken() // consume ")"

	var exceptions []string
	if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "throws" {
		p.nextToken()
		exceptions = p.parseTypeList()
	}

	cons := &ast.ConstructorDecl{
		Name:        name,
		Modifiers:   modifiers,
		Annotations: annotations,
		Parameters:  params,
		Exceptions:  exceptions,
		Range:       syntax.Range{Start: p.curTok.Range.Start},
	}

	p.emit(event.ConstructorDeclEvent{
		Name:       name,
		Modifiers:  modifiers,
		Parameters: params,
		Range:      cons.Range,
	})

	// Body skipping or block parsing
	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "{" {
		if p.signatureOnly || (p.policy&event.SubscribeOccurrences) == 0 {
			p.skipBalanced('{', '}')
		} else {
			cons.Body = p.parseBlock()
		}
	} else {
		p.reportError("Expected '{' after constructor signature", p.curTok.Range)
	}

	cons.Range.End = p.curTok.Range.Start
	return cons
}

func (p *Parser) parseBlock() *ast.Block {
	start := p.curTok.Range.Start
	p.nextToken() // consume "{"

	block := &ast.Block{
		Range: syntax.Range{Start: start},
	}

	p.emit(event.ScopeEnterEvent{Range: block.Range})

	for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != "}" {
		if p.curTok.Type == lexer.TokEOF || p.hasError {
			break
		}

		// Look for local variable declarations (like "int x = 10;" or "List<String> list;")
		isGeneric := p.isTypeToken(p.curTok) && p.peekTok.Type == lexer.TokSymbol && p.peekTok.Literal == "<"
		if (p.isTypeToken(p.curTok) && p.peekTok.Type == lexer.TokIdentifier) || isGeneric {
			typ := p.parseType()
			if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "<" {
				p.skipBalanced('<', '>')
			}
			if p.curTok.Type == lexer.TokIdentifier {
				name := p.curTok.Literal
				declRange := p.curTok.Range
				p.nextToken() // consume identifier

				// Skip initializer expression up to ';'
				if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "=" {
					p.nextToken()
					for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != ";" {
						if p.curTok.Type == lexer.TokEOF {
							break
						}
						p.nextToken()
					}
				}

				if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == ";" {
					p.nextToken() // consume ";"
				}

				localDecl := &ast.LocalVarDecl{
					Name:  name,
					Type:  typ,
					Range: declRange,
				}
				block.Locals = append(block.Locals, localDecl)

				p.emit(event.LocalDeclEvent{
					Name:     name,
					DataType: typ,
					Range:    declRange,
				})
			} else {
				p.nextToken()
			}
		} else if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "{" {
			// Nested block
			nested := p.parseBlock()
			block.Locals = append(block.Locals, nested)
		} else {
			p.nextToken()
		}
	}

	if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "}" {
		p.nextToken() // consume "}"
	}

	block.Range.End = p.curTok.Range.Start
	p.emit(event.ScopeLeaveEvent{Range: block.Range})

	return block
}

func (p *Parser) parseParameters() []*syntax.Parameter {
	var params []*syntax.Parameter
	for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != ")" {
		if p.curTok.Type == lexer.TokEOF {
			break
		}
		if p.curTok.Type == lexer.TokSymbol && (p.curTok.Literal == "{" || p.curTok.Literal == ";") {
			p.reportError("Unclosed parameter list: expected ')'", p.curTok.Range)
			break
		}
		// Read optional annotations/modifiers
		p.parseAnnotations()
		p.parseModifiers()

		typ := p.parseType()
		if p.curTok.Type == lexer.TokIdentifier {
			name := p.curTok.Literal
			paramRange := p.curTok.Range
			p.nextToken()

			params = append(params, &syntax.Parameter{
				Name:  name,
				Type:  typ,
				Range: paramRange,
			})
		}

		if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "," {
			p.nextToken() // consume ","
		}
	}
	return params
}

func (p *Parser) parseType() string {
	typ := p.parseQualifiedName()
	// Parse array modifiers (e.g., "String[]", "int[][]")
	for p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "[" {
		p.nextToken() // consume "["
		if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "]" {
			p.nextToken() // consume "]"
			typ += "[]"
		}
	}
	return typ
}

func (p *Parser) parseTypeList() []string {
	var types []string
	types = append(types, p.parseType())
	for p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "," {
		p.nextToken() // consume ","
		types = append(types, p.parseType())
	}
	return types
}

func (p *Parser) parseQualifiedName() string {
	name := p.curTok.Literal
	p.nextToken() // consume first identifier
	for p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "." {
		p.nextToken() // consume "."
		if p.curTok.Type == lexer.TokIdentifier {
			name += "." + p.curTok.Literal
			p.nextToken()
		}
	}
	return name
}

func (p *Parser) parseAnnotations() []*ast.Annotation {
	var annotations []*ast.Annotation
	for p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "@" {
		start := p.curTok.Range.Start
		p.nextToken() // consume "@"
		if p.curTok.Type == lexer.TokIdentifier {
			name := p.curTok.Literal
			p.nextToken() // consume name
			// skip potential annotation arguments, e.g., (value = "foo", count = 1)
			if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "(" {
				p.skipBalanced('(', ')')
			}
			end := p.curTok.Range.Start
			annotations = append(annotations, &ast.Annotation{
				Name:  name,
				Range: syntax.Range{Start: start, End: end},
			})
		}
	}
	return annotations
}

func (p *Parser) parseModifiers() []string {
	var modifiers []string
	for p.curTok.Type == lexer.TokKeyword && isModifier(p.curTok.Literal) {
		modifiers = append(modifiers, p.curTok.Literal)
		p.nextToken()
	}
	return modifiers
}

func (p *Parser) parseGenerics() []*ast.TypeParam {
	p.nextToken() // consume "<"
	var params []*ast.TypeParam
	for p.curTok.Type != lexer.TokSymbol || p.curTok.Literal != ">" {
		if p.curTok.Type == lexer.TokEOF {
			break
		}
		if p.curTok.Type == lexer.TokIdentifier {
			name := p.curTok.Literal
			start := p.curTok.Range.Start
			p.nextToken()
			var bounds []string
			if p.curTok.Type == lexer.TokKeyword && p.curTok.Literal == "extends" {
				p.nextToken()
				bounds = p.parseTypeList()
			}
			params = append(params, &ast.TypeParam{
				Name:   name,
				Bounds: bounds,
				Range:  syntax.Range{Start: start, End: p.curTok.Range.Start},
			})
		}
		if p.curTok.Type == lexer.TokSymbol && p.curTok.Literal == "," {
			p.nextToken()
		}
	}
	p.nextToken() // consume ">"
	return params
}

func (p *Parser) skipBalanced(open, close rune) {
	depth := 1
	p.nextToken() // consume opening symbol
	for depth > 0 && p.curTok.Type != lexer.TokEOF {
		if p.curTok.Type == lexer.TokSymbol {
			if p.curTok.Literal == string(open) {
				depth++
			} else if p.curTok.Literal == string(close) {
				depth--
			}
		}
		p.nextToken()
	}
}

func (p *Parser) synchronizeMember() {
	for {
		tok := p.peekTok
		if tok.Type == lexer.TokEOF {
			break
		}
		if tok.Literal == ";" || isModifier(tok.Literal) || tok.Literal == "@" || tok.Literal == "class" {
			p.nextToken() // move current token to synchronizing boundary
			break
		}
		p.nextToken()
	}
}

func isModifier(s string) bool {
	modifiers := map[string]bool{
		"public":       true,
		"private":      true,
		"protected":    true,
		"static":       true,
		"final":        true,
		"abstract":     true,
		"transient":    true,
		"volatile":     true,
		"synchronized": true,
		"native":       true,
		"strictfp":     true,
	}
	return modifiers[s]
}

func (p *Parser) isTypeToken(tok lexer.Token) bool {
	return tok.Type == lexer.TokIdentifier || (tok.Type == lexer.TokKeyword && isPrimitiveType(tok.Literal))
}

func isPrimitiveType(s string) bool {
	primitives := map[string]bool{
		"boolean": true,
		"byte":    true,
		"char":    true,
		"short":   true,
		"int":     true,
		"long":    true,
		"float":   true,
		"double":  true,
		"void":    true,
	}
	return primitives[s]
}
