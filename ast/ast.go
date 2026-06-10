package ast

import "github.com/kamichidu/go-javaparser/syntax"

// Node represents a generic node in our Declaration-Oriented AST.
type Node interface {
	GetRange() syntax.Range
}

// File represents a single Java source file's declaration structure.
type File struct {
	Path    string
	Package *Package
	Imports []*Import
	Types   []*TypeDecl
	Range   syntax.Range
}

func (f *File) GetRange() syntax.Range { return f.Range }

// Package represents a package declaration in the Java file.
type Package struct {
	Name  string
	Range syntax.Range
}

func (p *Package) GetRange() syntax.Range { return p.Range }

// Import represents a single import statement.
type Import struct {
	Name     string
	IsStatic bool
	IsStar   bool
	Range    syntax.Range
}

func (i *Import) GetRange() syntax.Range { return i.Range }

// MemberNode is the interface satisfied by any member inside a Type declaration.
type MemberNode interface {
	Node
	isMember()
}

// TypeDecl represents a class, interface, enum, record, or annotation type declaration.
type TypeDecl struct {
	Kind        syntax.TypeKind
	Name        string
	Modifiers   []string
	Annotations []*Annotation
	Generics    []*TypeParam
	Extends     []string
	Implements  []string
	Members     []MemberNode
	Range       syntax.Range
}

func (t *TypeDecl) GetRange() syntax.Range { return t.Range }
func (t *TypeDecl) isMember()              {} // Can be a nested type

// FieldDecl represents a field or enum constant declaration.
type FieldDecl struct {
	Name        string
	Type        string
	Modifiers   []string
	Annotations []*Annotation
	IsEnumConst bool
	Range       syntax.Range
}

func (f *FieldDecl) GetRange() syntax.Range { return f.Range }
func (f *FieldDecl) isMember()              {}

// MethodDecl represents a method declaration.
type MethodDecl struct {
	Name        string
	ReturnType  string
	Modifiers   []string
	Annotations []*Annotation
	Generics    []*TypeParam
	Parameters  []*syntax.Parameter
	Exceptions  []string
	Body        *Block
	Range       syntax.Range
}

func (m *MethodDecl) GetRange() syntax.Range { return m.Range }
func (m *MethodDecl) isMember()              {}

// ConstructorDecl represents a constructor declaration.
type ConstructorDecl struct {
	Name        string
	Modifiers   []string
	Annotations []*Annotation
	Parameters  []*syntax.Parameter
	Exceptions  []string
	Body        *Block
	Range       syntax.Range
}

func (c *ConstructorDecl) GetRange() syntax.Range { return c.Range }
func (c *ConstructorDecl) isMember()              {}

// LocalVarDecl represents a local variable declared in a block.
type LocalVarDecl struct {
	Name      string
	Type      string
	Modifiers []string
	Range     syntax.Range
}

func (l *LocalVarDecl) GetRange() syntax.Range { return l.Range }

// Block represents a sequence of local declarations or statements.
type Block struct {
	Locals []Node // Can contain nested Blocks or LocalVarDecls
	Range  syntax.Range
}

func (b *Block) GetRange() syntax.Range { return b.Range }

// Annotation represents an annotation used on a declaration (e.g. "@Override").
type Annotation struct {
	Name  string
	Range syntax.Range
}

func (a *Annotation) GetRange() syntax.Range { return a.Range }

// TypeParam represents a generic type parameter (e.g. "<T extends Comparable>").
type TypeParam struct {
	Name   string
	Bounds []string
	Range  syntax.Range
}

func (tp *TypeParam) GetRange() syntax.Range { return tp.Range }
