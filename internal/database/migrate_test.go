package database

import (
	"fmt"
	"strings"
	"testing"
)

func TestMigrationFiles_OrderedByVersion(t *testing.T) {
	files, err := migrationFiles()
	if err != nil {
		t.Fatalf("migrationFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one embedded migration")
	}

	for i, f := range files {
		if !strings.HasPrefix(f.filename, fmt.Sprintf("%03d_", f.version)) {
			t.Fatalf("file %q violates the NNN_description.sql naming convention", f.filename)
		}
		if f.contents == "" {
			t.Fatalf("migration %q is empty", f.filename)
		}
		if i > 0 && files[i-1].version >= f.version {
			t.Fatalf("migrations not strictly ordered: %d then %d", files[i-1].version, f.version)
		}
	}
}
