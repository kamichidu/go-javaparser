package completion

import (
	"testing"

	"github.com/kamichidu/go-javaparser/syntax"
)

func TestExtractContextQualifiedName(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        System.out.
    }
}`
	// Position is after "System.out." (Line 4, Col 20, 1-based)
	pos := syntax.Position{Line: 4, Col: 20}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access context, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver to be parsed, got nil")
	}
	if ctx.Receiver.RawText != "System.out" {
		t.Errorf("expected receiver rawText 'System.out', got %q", ctx.Receiver.RawText)
	}
	if ctx.Receiver.Kind != ReceiverQualifiedName {
		t.Errorf("expected receiver kind qualified_name, got %s", ctx.Receiver.Kind)
	}
}

func TestExtractContextNewExpression(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        new ArrayList<>().
    }
}`
	// Position is after "new ArrayList<>()." (Line 4, Col 27)
	pos := syntax.Position{Line: 4, Col: 27}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access context, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver, got nil")
	}
	if ctx.Receiver.RawText != "new ArrayList<>()" {
		t.Errorf("expected receiver rawText 'new ArrayList<>()', got %q", ctx.Receiver.RawText)
	}
	if ctx.Receiver.Kind != ReceiverNewExpression {
		t.Errorf("expected receiver kind new_expression, got %s", ctx.Receiver.Kind)
	}
}

func TestExtractContextSimpleNameWithLocals(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        List<String> list;
        list.
    }
}`
	// Position is after "list." (Line 5, Col 14)
	pos := syntax.Position{Line: 5, Col: 14}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver, got nil")
	}
	if ctx.Receiver.RawText != "list" {
		t.Errorf("expected receiver 'list', got %q", ctx.Receiver.RawText)
	}
	if ctx.Receiver.Kind != ReceiverSimpleName {
		t.Errorf("expected receiver kind simple_name, got %s", ctx.Receiver.Kind)
	}

	// Verify local declarations contains list with type List<String>
	found := false
	for _, l := range ctx.Locals {
		if l.Name == "list" {
			found = true
			if l.Type != "List<String>" && l.Type != "List" { // Depending on simplified parser
				t.Errorf("expected local var 'list' type 'List<String>', got %q", l.Type)
			}
		}
	}
	if !found {
		t.Errorf("expected local declaration 'list' to be captured in context, locals: %v", ctx.Locals)
	}
}

func TestExtractContextMethodInvocationChain(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        foo.bar().baz().
    }
}`
	// Position is after "foo.bar().baz()." (Line 4, Col 25)
	pos := syntax.Position{Line: 4, Col: 25}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver, got nil")
	}
	if ctx.Receiver.RawText != "foo.bar().baz()" {
		t.Errorf("expected receiver 'foo.bar().baz()', got %q", ctx.Receiver.RawText)
	}
	if ctx.Receiver.Kind != ReceiverMethodInvocation {
		t.Errorf("expected receiver kind method_invocation, got %s", ctx.Receiver.Kind)
	}
}

func TestExtractContextMemberPrefix(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        System.out.f
    }
}`
	// Position is after "System.out.f" (Line 4, Col 21, 1-based)
	pos := syntax.Position{Line: 4, Col: 21}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver, got nil")
	}
	if ctx.Receiver.RawText != "System.out" {
		t.Errorf("expected receiver rawText 'System.out', got %q", ctx.Receiver.RawText)
	}
	if ctx.MemberPrefix != "f" {
		t.Errorf("expected memberPrefix 'f', got %q", ctx.MemberPrefix)
	}
}

func TestExtractContextMethodInvocationChainDot(t *testing.T) {
	content := `package com.example;
public class Main {
    void run() {
        System.out.format().
    }
}`
	// Position is after "System.out.format()." (Line 4, Col 29, 1-based)
	pos := syntax.Position{Line: 4, Col: 29}
	ctx := ExtractContext(content, "Main.java", pos)

	if ctx.Kind != ContextMemberAccess {
		t.Fatalf("expected member_access, got %s", ctx.Kind)
	}
	if ctx.Receiver == nil {
		t.Fatalf("expected receiver, got nil")
	}
	if ctx.Receiver.RawText != "System.out.format()" {
		t.Errorf("expected receiver 'System.out.format()', got %q", ctx.Receiver.RawText)
	}
	if ctx.Receiver.Kind != ReceiverMethodInvocation {
		t.Errorf("expected receiver kind method_invocation, got %s", ctx.Receiver.Kind)
	}
}
