package syntax

// Position represents a 1-based line and column coordinate in the source file.
type Position struct {
	Line int // 1-based
	Col  int // 1-based
}

// Range represents a span in the source file between a start position (inclusive)
// and an end position (exclusive/inclusive depending on context, typically inclusive).
type Range struct {
	Start Position
	End   Position
}

// TypeKind represents the categories of Java types as defined in JLS.
type TypeKind string

const (
	KindClass          TypeKind = "class"
	KindInterface      TypeKind = "interface"
	KindEnum           TypeKind = "enum"
	KindRecord         TypeKind = "record"
	KindAnnotationType TypeKind = "annotation"
)

// Parameter represents a descriptor for a method or constructor parameter.
type Parameter struct {
	Name        string
	Type        string // Simplified type name/descriptor text
	Annotations []string
	Modifiers   []string
	Range       Range
}
