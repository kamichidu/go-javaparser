package parser

import (
	"testing"

	"github.com/kamichidu/go-javaparser/ast"
	"github.com/kamichidu/go-javaparser/event"
	"github.com/kamichidu/go-javaparser/lexer"
	"github.com/kamichidu/go-javaparser/syntax"
)

// mockSink captures emitted events for verification.
type mockSink struct {
	events []event.SourceEvent
}

func (s *mockSink) Emit(evt event.SourceEvent) error {
	s.events = append(s.events, evt)
	return nil
}

func TestParserComplete(t *testing.T) {
	input := `package com.example;
import java.util.List;

@Deprecated
public class Main {
    private String name = "test";

    public Main(String name) {
        int x = 100;
    }

    public void run() {
        String msg = "running";
    }
}`

	// 1. Test AST & Event Projections with Occurrences Enabled (Full Parse)
	l := lexer.New(input)
	sink := &mockSink{}
	p := New(l, "Main.java", WithEventSink(sink), WithPolicy(event.SubscribeOccurrences))

	file, err := p.ParseFile()
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify AST Projection
	if file.Package.Name != "com.example" {
		t.Errorf("Expected package com.example, got %q", file.Package.Name)
	}
	if len(file.Imports) != 1 || file.Imports[0].Name != "java.util.List" {
		t.Errorf("Expected import java.util.List, got %v", file.Imports)
	}
	if len(file.Types) != 1 || file.Types[0].Name != "Main" {
		t.Fatalf("Expected type Main, got %v", file.Types)
	}

	mainType := file.Types[0]
	if mainType.Kind != syntax.KindClass {
		t.Errorf("Expected main type kind class, got %q", mainType.Kind)
	}
	if len(mainType.Annotations) != 1 || mainType.Annotations[0].Name != "Deprecated" {
		t.Errorf("Expected @Deprecated annotation, got %v", mainType.Annotations)
	}

	// Verify Method & Constructor bodies are parsed in Full Mode
	if len(mainType.Members) != 3 {
		t.Fatalf("Expected 3 members (field, constructor, method), got %d", len(mainType.Members))
	}

	field, ok := mainType.Members[0].(*ast.FieldDecl)
	if !ok || field.Name != "name" || field.Type != "String" {
		t.Errorf("Expected field name, got %v", mainType.Members[0])
	}

	constructor, ok := mainType.Members[1].(*ast.ConstructorDecl)
	if !ok || constructor.Name != "Main" || constructor.Body == nil {
		t.Errorf("Expected constructor Main with parsed body, got %v", mainType.Members[1])
	}
	if len(constructor.Body.Locals) != 1 {
		t.Errorf("Expected 1 local var in constructor, got %d", len(constructor.Body.Locals))
	}

	method, ok := mainType.Members[2].(*ast.MethodDecl)
	if !ok || method.Name != "run" || method.Body == nil {
		t.Errorf("Expected method run with parsed body, got %v", mainType.Members[2])
	}
	if len(method.Body.Locals) != 1 {
		t.Errorf("Expected 1 local var in method run, got %d", len(method.Body.Locals))
	}

	// Verify Event Projection (on mockSink)
	var eventTypes []event.EventType
	for _, e := range sink.events {
		eventTypes = append(eventTypes, e.Type())
	}

	expectedEvents := []event.EventType{
		"StartFile",
		"PackageDecl",
		"ImportDecl",
		"TypeDecl",
		"FieldDecl",
		"ConstructorDecl",
		"ScopeEnter",
		"LocalDecl",
		"ScopeLeave",
		"MethodDecl",
		"ScopeEnter",
		"LocalDecl",
		"ScopeLeave",
		"EndFile",
	}

	if len(eventTypes) != len(expectedEvents) {
		t.Fatalf("Expected %d events, got %d:\nExpected: %v\nGot: %v", len(expectedEvents), len(eventTypes), expectedEvents, eventTypes)
	}

	for i, et := range eventTypes {
		if et != expectedEvents[i] {
			t.Errorf("Event index %d mismatch: expected=%s, got=%s", i, expectedEvents[i], et)
		}
	}
}

func TestParserBodySkipping(t *testing.T) {
	input := `package com.example;
public class Main {
    public Main() {
        int x = 100;
    }
    public void run() {
        String msg = "running";
    }
}`

	// 2. Test On-Demand Body Skipping (Occurrences NOT subscribed / DefaultPolicy only)
	l := lexer.New(input)
	sink := &mockSink{}
	// Running with default policy (without SubscribeOccurrences) means bodies conceptually don't exist
	p := New(l, "Main.java", WithEventSink(sink), WithPolicy(event.DefaultPolicy))

	file, err := p.ParseFile()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	mainType := file.Types[0]
	constructor := mainType.Members[0].(*ast.ConstructorDecl)
	method := mainType.Members[1].(*ast.MethodDecl)

	// In skipping mode, AST bodies should be completely nil!
	if constructor.Body != nil {
		t.Errorf("Expected skipped constructor body (nil), got %v", constructor.Body)
	}
	if method.Body != nil {
		t.Errorf("Expected skipped method body (nil), got %v", method.Body)
	}

	// In skipping mode, no local variable declarations should be emitted!
	for _, e := range sink.events {
		if e.Type() == "LocalDecl" || e.Type() == "ScopeEnter" || e.Type() == "ScopeLeave" {
			t.Errorf("Unexpected local scope event leaked: %s", e.Type())
		}
	}
}

func TestConsumerDrivenRangeParsing(t *testing.T) {
	// The consumer isolated a method body block and wants to parse it directly
	input := `{
        int a = 1;
        String b = "test";
    }`

	l := lexer.New(input)
	sink := &mockSink{}
	p := New(l, "DeltaScope", WithEventSink(sink), WithPolicy(event.SubscribeOccurrences))

	block, err := p.ParseLocalScope("M:com.example.Main#run()V")
	if err != nil {
		t.Fatalf("Failed local scope parse: %v", err)
	}

	if len(block.Locals) != 2 {
		t.Fatalf("Expected 2 local variables inside block, got %d", len(block.Locals))
	}

	v1 := block.Locals[0].(*ast.LocalVarDecl)
	if v1.Name != "a" || v1.Type != "int" {
		t.Errorf("Expected var 'a' of type 'int', got %s %s", v1.Type, v1.Name)
	}

	v2 := block.Locals[1].(*ast.LocalVarDecl)
	if v2.Name != "b" || v2.Type != "String" {
		t.Errorf("Expected var 'b' of type 'String', got %s %s", v2.Type, v2.Name)
	}

	// Verify occurrences events were projected in order
	expected := []event.EventType{
		"StartDeltaScope",
		"ScopeEnter",
		"LocalDecl",
		"LocalDecl",
		"ScopeLeave",
		"EndDeltaScope",
	}

	if len(sink.events) != len(expected) {
		t.Fatalf("Expected %d events, got %d", len(expected), len(sink.events))
	}

	for i, e := range sink.events {
		if e.Type() != expected[i] {
			t.Errorf("[%d] expected event type %s, got %s", i, expected[i], e.Type())
		}
	}
}

func TestParserErrorRecovery(t *testing.T) {
	input := `public class Main {
    private int val = 10;
    
    // Broken method signature with missing parameter closing bracket
    public void broken(int x {
        System.out.println(x);
    }

    private String name = "recovery_success";
}`

	l := lexer.New(input)
	p := New(l, "Recovery.java")

	_, err := p.ParseFile()
	// Since file parsing failed because of broken syntax inside member, p.ParseFile() will return first error,
	// BUT because of synchronizeMember(), the internal AST members parsed successfully before error or skipped member
	// can be inspected, or we can check recovery list directly.
	if err == nil {
		t.Fatal("Expected compilation/parse error due to broken syntax")
	}

	// Let's check if the syntax error was captured correctly
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("Expected ParseError type, got %T", err)
	}
	if parseErr.Range.Start.Line != 5 {
		t.Errorf("Expected error near line 5, got near line %d", parseErr.Range.Start.Line)
	}
}
