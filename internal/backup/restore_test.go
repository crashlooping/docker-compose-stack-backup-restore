package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreFromBackup_InvalidArchive(t *testing.T) {
	tmp := t.TempDir()
	badFile := filepath.Join(tmp, "bad.txt")
	os.WriteFile(badFile, []byte("not an archive"), 0o644)
	err := RestoreFromBackup(badFile, RestoreOptions{TargetDir: tmp})
	if err == nil {
		t.Error("Expected error for invalid archive format")
	}
}
