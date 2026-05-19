package assistantsvc

import (
	"testing"
)

func TestRegistryRegisterAndList(t *testing.T) {
	r := NewRegistry()
	tools := r.List()
	if len(tools) < 4 {
		t.Fatalf("expected at least 4 default tools, got %d", len(tools))
	}

	r.Register(&Tool{Name: "custom", Description: "custom tool"})
	tools = r.List()
	found := false
	for _, tool := range tools {
		if tool.Name == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("custom tool not found after Register")
	}
}

func TestRegistryExecute(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute("explain_factor", "sma($close,20)")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	_, err = r.Execute("nonexistent", "")
	if err != nil {
		t.Fatal("expected nil error for nonexistent tool")
	}
}

func TestRegistrySetKB(t *testing.T) {
	r := NewRegistry()
	kb := NewKnowledgeBase()
	r.SetKB(kb)
	tools := r.List()
	found := false
	for _, t := range tools {
		if t.Name == "search_docs" {
			found = true
		}
	}
	if !found {
		t.Fatal("search_docs not registered after SetKB")
	}
}

func TestRegistryConcurrency(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			r.Register(&Tool{Name: "t", Description: "d"})
			r.List()
			r.Execute("explain_factor", "")
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
