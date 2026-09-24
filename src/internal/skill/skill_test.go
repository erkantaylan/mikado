package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextHasFrontmatterAndMarker(t *testing.T) {
	if !strings.HasPrefix(Text(), "---\nname: mikado\ndescription: ") {
		t.Fatalf("SKILL.md must start with Claude Code skill frontmatter, got %q", Text()[:40])
	}
	if !strings.Contains(Text(), marker) {
		t.Fatal("SKILL.md must carry the install marker so reinstalls can overwrite it")
	}
}

func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mikado")
	path, err := Install(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != Text() {
		t.Fatal("installed file differs from the embedded guide")
	}

	// An older install (it carries the marker) is replaced without --force.
	if err := os.WriteFile(path, []byte("old guide, "+marker), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("reinstall over our own file: %v", err)
	}

	// A file someone else wrote is refused, unless forced.
	if err := os.WriteFile(path, []byte("hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, false); err == nil {
		t.Fatal("expected a refusal to overwrite a file mikado did not write")
	}
	if got, _ := os.ReadFile(path); string(got) != "hand-written" {
		t.Fatal("a refused install must leave the file alone")
	}
	if _, err := Install(dir, true); err != nil {
		t.Fatalf("forced install: %v", err)
	}
}
