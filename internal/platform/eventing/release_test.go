package eventing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseOrderAndTerminalPost(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	lines := executableLines(string(raw))
	if got := lines[len(lines)-1]; !strings.HasPrefix(got, "run_step POST ") {
		t.Fatalf("last release command = %q, want terminal POST", got)
	}

	order := []string{
		"PRECHECK",
		"BACKUP",
		"DB_MIGRATE",
		"BROKER_PROVISION",
		"SCHEMA_REGISTER",
		"DEPLOY_API",
		"DEPLOY_WORKER",
		"VERIFY",
		"ENABLE",
		"POST",
	}
	last := -1
	for _, step := range order {
		index := stepIndex(lines, step)
		if index <= last {
			t.Fatalf("release step %s is out of order", step)
		}
		last = index
	}
}

func TestEventingMigrationPrecedesFinalPostMigration(t *testing.T) {
	t.Parallel()

	migrations := filepath.Join("..", "..", "..", "cmd", "migrate", "migrations")
	if _, err := os.Stat(filepath.Join(migrations, "000005_eventing.up.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(migrations, "000006_create_posts.up.sql")); err != nil {
		t.Fatal(err)
	}
}

func TestEventingMigrationDefinesWatermarks(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", "migrate", "migrations", "000005_eventing.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "event_inbox_watermarks") {
		t.Fatal("eventing migration must create inbox watermarks")
	}
}

func executableLines(raw string) []string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func stepIndex(lines []string, step string) int {
	prefix := "run_step " + step + " "
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return i
		}
	}
	return -1
}
