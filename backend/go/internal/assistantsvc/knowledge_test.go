package assistantsvc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnowledgeBaseLoad(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "test.md")
	os.WriteFile(f1, []byte("# Test Doc\ncontent here\n"), 0644)
	f2 := filepath.Join(dir, "sub", "nested.md")
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(f2, []byte("# Nested Doc\nnested content\n"), 0644)

	kb := NewKnowledgeBase()
	if err := kb.Load(dir); err != nil {
		t.Fatal(err)
	}
	if kb.Status() != "ready" {
		t.Fatal("expected ready status after Load")
	}
}

func TestKnowledgeBaseSearch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("# Alpha\nthis is about risk management\n"), 0644)
	os.WriteFile(filepath.Join(dir, "beta.md"), []byte("# Beta\nthis is about market data\n"), 0644)

	kb := NewKnowledgeBase()
	kb.Load(dir)

	results := kb.Search("risk")
	if len(results) == 0 {
		t.Fatal("expected results for 'risk'")
	}
	if results[0].Title != "Alpha" {
		t.Fatalf("expected Alpha, got %s", results[0].Title)
	}

	results = kb.Search("market")
	if len(results) == 0 {
		t.Fatal("expected results for 'market'")
	}
}

func TestKnowledgeBaseSearchEmpty(t *testing.T) {
	kb := NewKnowledgeBase()
	if kb.Status() != "not_loaded" {
		t.Fatal("expected not_loaded for new KB")
	}
	results := kb.Search("anything")
	if len(results) != 0 {
		t.Fatal("expected empty results for unloaded KB")
	}
}

func TestKnowledgeBaseLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	kb := NewKnowledgeBase()
	if err := kb.Load(dir); err != nil {
		t.Fatal(err)
	}
	if kb.Status() != "ready" {
		t.Fatal("expected ready for empty dir")
	}
	results := kb.Search("anything")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestKnowledgeBaseLoadSkipNonMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("not markdown"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Readme\nhello\n"), 0644)

	kb := NewKnowledgeBase()
	kb.Load(dir)
	results := kb.Search("hello")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
