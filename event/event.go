package event

import "github.com/kamichidu/go-javaparser/syntax"

type EventType string

// SourceEvent is the common interface implemented by all streamed elements.
type SourceEvent interface {
	Type() EventType
	GetRange() syntax.Range
}

// EventSink represents a subscriber's callback interface to receive sequential events.
type EventSink interface {
	Emit(evt SourceEvent) error
}

// SubscriptionPolicy defines which event categories are enabled.
type SubscriptionPolicy uint32

const (
	// DefaultPolicy always streams declaration events (Package, Import, Type, Member, etc.)
	DefaultPolicy SubscriptionPolicy = 1 << iota

	// SubscribeOccurrences enables tracking of local variables and reference occurrences.
	SubscribeOccurrences
)

// StartFileEvent represents the beginning of parsing for a file.
type StartFileEvent struct {
	SourceID string
}

func (e StartFileEvent) Type() EventType        { return "StartFile" }
func (e StartFileEvent) GetRange() syntax.Range { return syntax.Range{} }

// EndFileEvent represents the successful end of parsing for a file.
type EndFileEvent struct {
	SourceID string
}

func (e EndFileEvent) Type() EventType        { return "EndFile" }
func (e EndFileEvent) GetRange() syntax.Range { return syntax.Range{} }

// PackageDeclEvent represents the packages declared in the file.
type PackageDeclEvent struct {
	Name  string
	Range syntax.Range
}

func (e PackageDeclEvent) Type() EventType        { return "PackageDecl" }
func (e PackageDeclEvent) GetRange() syntax.Range { return e.Range }

// ImportDeclEvent represents an import statement.
type ImportDeclEvent struct {
	Name     string
	IsStatic bool
	IsStar   bool
	Range    syntax.Range
}

func (e ImportDeclEvent) Type() EventType        { return "ImportDecl" }
func (e ImportDeclEvent) GetRange() syntax.Range { return e.Range }

// TypeDeclEvent represents a class, interface, enum, record, or annotation type declaration.
type TypeDeclEvent struct {
	Kind        syntax.TypeKind
	Name        string
	Modifiers   []string
	Annotations []string
	Range       syntax.Range
}

func (e TypeDeclEvent) Type() EventType        { return "TypeDecl" }
func (e TypeDeclEvent) GetRange() syntax.Range { return e.Range }

// FieldDeclEvent represents fields and enum constants inside type declarations.
type FieldDeclEvent struct {
	Name        string
	DataType    string // Renamed from Type to avoid collision with method Type()
	Modifiers   []string
	IsEnumConst bool
	Range       syntax.Range
}

func (e FieldDeclEvent) Type() EventType        { return "FieldDecl" }
func (e FieldDeclEvent) GetRange() syntax.Range { return e.Range }

// MethodDeclEvent represents a method declaration.
type MethodDeclEvent struct {
	Name       string
	ReturnType string
	Modifiers  []string
	Parameters []*syntax.Parameter
	Range      syntax.Range
}

func (e MethodDeclEvent) Type() EventType        { return "MethodDecl" }
func (e MethodDeclEvent) GetRange() syntax.Range { return e.Range }

// ConstructorDeclEvent represents a constructor declaration.
type ConstructorDeclEvent struct {
	Name       string
	Modifiers  []string
	Parameters []*syntax.Parameter
	Range      syntax.Range
}

func (e ConstructorDeclEvent) Type() EventType        { return "ConstructorDecl" }
func (e ConstructorDeclEvent) GetRange() syntax.Range { return e.Range }

// StartDeltaScopeEvent marks the beginning of a local range/scope partial re-parse.
type StartDeltaScopeEvent struct {
	SourceID   string
	MergeScope string // identifier key (e.g. method JVM signature)
	Range      syntax.Range
}

func (e StartDeltaScopeEvent) Type() EventType        { return "StartDeltaScope" }
func (e StartDeltaScopeEvent) GetRange() syntax.Range { return e.Range }

// EndDeltaScopeEvent marks the successful end of a local range/scope partial re-parse.
type EndDeltaScopeEvent struct {
	SourceID   string
	MergeScope string
	Range      syntax.Range
}

func (e EndDeltaScopeEvent) Type() EventType        { return "EndDeltaScope" }
func (e EndDeltaScopeEvent) GetRange() syntax.Range { return e.Range }

// ScopeEnterEvent marks entry into a local block scope (braces).
type ScopeEnterEvent struct {
	Range syntax.Range
}

func (e ScopeEnterEvent) Type() EventType        { return "ScopeEnter" }
func (e ScopeEnterEvent) GetRange() syntax.Range { return e.Range }

// ScopeLeaveEvent marks departure from a local block scope.
type ScopeLeaveEvent struct {
	Range syntax.Range
}

func (e ScopeLeaveEvent) Type() EventType        { return "ScopeLeave" }
func (e ScopeLeaveEvent) GetRange() syntax.Range { return e.Range }

// LocalDeclEvent represents a local variable declaration or parameter inside a block.
type LocalDeclEvent struct {
	Name     string
	DataType string // Renamed from Type to avoid collision with method Type()
	Range    syntax.Range
}

func (e LocalDeclEvent) Type() EventType        { return "LocalDecl" }
func (e LocalDeclEvent) GetRange() syntax.Range { return e.Range }

// ParseErrorEvent reports a syntax error encountered during parsing.
type ParseErrorEvent struct {
	SourceID string
	Message  string
	Range    syntax.Range
}

func (e ParseErrorEvent) Type() EventType        { return "ParseError" }
func (e ParseErrorEvent) GetRange() syntax.Range { return e.Range }
