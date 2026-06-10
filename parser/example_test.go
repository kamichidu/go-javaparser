package parser_test

import (
	"fmt"

	"github.com/kamichidu/go-javaparser/event"
	"github.com/kamichidu/go-javaparser/lexer"
	"github.com/kamichidu/go-javaparser/parser"
)

type printSink struct{}

func (s *printSink) Emit(evt event.SourceEvent) error {
	switch e := evt.(type) {
	case event.PackageDeclEvent:
		fmt.Printf("Event: package declared - %s\n", e.Name)
	case event.TypeDeclEvent:
		fmt.Printf("Event: type declared - %s (%s)\n", e.Name, e.Kind)
	case event.FieldDeclEvent:
		fmt.Printf("Event: field declared - %s (type: %s)\n", e.Name, e.DataType)
	}
	return nil
}

func Example() {
	source := `package com.example;
	public class Utils {
		private String version = "1.0.0";
	}`

	l := lexer.New(source)
	sink := &printSink{}
	p := parser.New(l, "Utils.java", parser.WithEventSink(sink))

	file, err := p.ParseFile()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("AST: Parsed package - %s\n", file.Package.Name)
	fmt.Printf("AST: Parsed type - %s\n", file.Types[0].Name)

	// Output:
	// Event: package declared - com.example
	// Event: type declared - Utils (class)
	// Event: field declared - version (type: String)
	// AST: Parsed package - com.example
	// AST: Parsed type - Utils
}
