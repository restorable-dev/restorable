package recipes

import (
	"os"
	"path/filepath"
	"testing"
)

// TestShippedRecipesParse guards the recipe files shipped in /recipes at the
// repo root: they must always parse and validate.
func TestShippedRecipesParse(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "recipes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read shipped recipes dir: %v", err)
	}
	found := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		found++
		t.Run(e.Name(), func(t *testing.T) {
			rec, err := Load(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("shipped recipe does not parse: %v", err)
			}
			if len(rec.Checks) == 0 {
				t.Error("shipped recipe has no checks")
			}
		})
	}
	if found < 3 {
		t.Errorf("expected at least 3 shipped recipes, found %d", found)
	}
}
