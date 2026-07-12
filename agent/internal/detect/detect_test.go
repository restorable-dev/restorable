package detect

import (
	"strings"
	"testing"

	"github.com/restorable-dev/restorable/agent/internal/recipes"
)

// parses asserts the generated recipe is valid per the real recipe parser —
// the whole point of init is that its output actually works.
func parses(t *testing.T, yaml string) *recipes.Recipe {
	t.Helper()
	rec, err := recipes.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("generated recipe does not parse:\n%s\nerror: %v", yaml, err)
	}
	return rec
}

func TestAnalyzeNextcloud(t *testing.T) {
	files := []string{"config/config.php", "config/config.sample.php", "data/owncloud.db"}
	for i := 0; i < 120; i++ {
		files = append(files, "data/files/photo"+string(rune('a'+i%26))+".jpg")
	}
	sug := Analyze("nextcloud", files)
	if sug.RecipeName != "nextcloud" {
		t.Errorf("name = %q, want nextcloud", sug.RecipeName)
	}
	rec := parses(t, sug.RecipeYAML)
	if len(rec.Checks) == 0 {
		t.Fatal("no checks generated")
	}
	// owncloud.db should trigger a sqlite check.
	if !strings.Contains(sug.RecipeYAML, "type: sqlite") {
		t.Errorf("expected a sqlite check for owncloud.db:\n%s", sug.RecipeYAML)
	}
	// config.php should be a required file.
	if !strings.Contains(sug.RecipeYAML, "config/config.php") {
		t.Errorf("expected config.php required:\n%s", sug.RecipeYAML)
	}
	if !findingContains(sug, "Nextcloud") {
		t.Errorf("findings should mention Nextcloud: %v", sug.Findings)
	}
}

func TestAnalyzeVaultwarden(t *testing.T) {
	files := []string{"db.sqlite3", "rsa_key.pem", "config.json", "attachments/x/file"}
	sug := Analyze("vaultwarden-data", files)
	if sug.RecipeName != "vaultwarden" {
		t.Errorf("name = %q, want vaultwarden", sug.RecipeName)
	}
	parses(t, sug.RecipeYAML)
	if strings.Count(sug.RecipeYAML, "type: sqlite") != 1 {
		t.Errorf("expected one sqlite check for db.sqlite3:\n%s", sug.RecipeYAML)
	}
}

func TestAnalyzeGenericSQLiteApp(t *testing.T) {
	// The Open WebUI shape: a webui.db plus data dirs.
	files := []string{"webui.db", "cache/x", "uploads/a", "uploads/b", "vector_db/index"}
	sug := Analyze("data", files)
	parses(t, sug.RecipeYAML)
	if !strings.Contains(sug.RecipeYAML, "path: webui.db") {
		t.Errorf("expected sqlite check for webui.db:\n%s", sug.RecipeYAML)
	}
}

func TestAnalyzeDumpIsCommented(t *testing.T) {
	files := []string{"backup/db.sql.gz", "app.conf"}
	sug := Analyze("myapp", files)
	rec := parses(t, sug.RecipeYAML)
	// The dump suggestion must be commented, so the first test stays green
	// and dependency-free — no active postgres check.
	for _, c := range rec.Checks {
		if c.TypeName() == "postgres" {
			t.Error("postgres check should be commented out, not active")
		}
	}
	if !strings.Contains(sug.RecipeYAML, "# - type: postgres") {
		t.Errorf("expected commented postgres suggestion:\n%s", sug.RecipeYAML)
	}
}

func TestAnalyzeGenericFallback(t *testing.T) {
	files := []string{"srv/a.txt", "srv/b.txt", "srv/c.txt", "important.conf"}
	sug := Analyze("myapp", files)
	if sug.RecipeName != "myapp" {
		t.Errorf("name = %q, want myapp", sug.RecipeName)
	}
	rec := parses(t, sug.RecipeYAML)
	if rec.Checks[0].TypeName() != "files" {
		t.Errorf("first check should be files, got %s", rec.Checks[0].TypeName())
	}
	if !strings.Contains(sug.RecipeYAML, "min_files:") {
		t.Errorf("expected a min_files assertion on a directory:\n%s", sug.RecipeYAML)
	}
}

func TestAnalyzeEmptyBackupStillValid(t *testing.T) {
	sug := Analyze("empty", nil)
	// Must still parse (checksum_sample keeps it valid) — and on a real run it
	// will fail loudly, which is the correct signal for an empty backup.
	parses(t, sug.RecipeYAML)
	if !strings.Contains(sug.RecipeYAML, "checksum_sample:") {
		t.Errorf("empty backup recipe must keep a checksum_sample:\n%s", sug.RecipeYAML)
	}
}

func TestSlugSanitizesName(t *testing.T) {
	sug := Analyze("My App (Prod)!", []string{"a.txt"})
	if strings.ContainsAny(sug.RecipeName, " ()!") {
		t.Errorf("name %q not slugified", sug.RecipeName)
	}
}

func findingContains(s Suggestion, sub string) bool {
	for _, f := range s.Findings {
		if strings.Contains(f, sub) {
			return true
		}
	}
	return false
}
