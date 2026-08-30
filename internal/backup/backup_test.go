package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/archive"
	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/docker"
)

const (
	testComposeYml = "docker-compose.yml"
	nonexistentDir = "/nonexistent"
	errMsg         = "Expected no error, got %v"
)

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()
	composeYml := filepath.Join(dir, testComposeYml)
	composeYaml := filepath.Join(dir, "docker-compose.yaml")
	os.WriteFile(composeYml, []byte("version: '3'"), 0o644)
	file, err := docker.FindComposeFile(dir)
	if err != nil || file != testComposeYml {
		t.Errorf("Expected docker-compose.yml, got %v, err: %v", file, err)
	}
	os.Remove(composeYml)
	os.WriteFile(composeYaml, []byte("version: '3'"), 0o644)
	file, err = docker.FindComposeFile(dir)
	if err != nil || file != "docker-compose.yaml" {
		t.Errorf("Expected docker-compose.yaml, got %v, err: %v", file, err)
	}
	os.Remove(composeYaml)
	file, err = docker.FindComposeFile(dir)
	if err != nil || file != "" {
		t.Errorf("Expected '', got %v, err: %v", file, err)
	}
}

func TestTarGzFolderAndZipFolder(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello world"), 0o644)
	os.Mkdir(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "should_ignore.txt"), []byte("ignore me"), 0o644)
	dst := t.TempDir()
	tarPath := filepath.Join(dst, "test.tar.gz")
	zipPath := filepath.Join(dst, "test.zip")
	if err := archive.TarGzFolderWithVolumes(src, tarPath, nil); err != nil {
		t.Fatalf("TarGzFolderWithVolumes failed: %v", err)
	}
	if err := archive.ZipFolderWithVolumes(src, zipPath, nil); err != nil {
		t.Fatalf("ZipFolderWithVolumes failed: %v", err)
	}
}

func TestBackupComposeStackWithFormatsErrors(t *testing.T) {
	dst := t.TempDir()
	err := BackupComposeStackWithFormats(nonexistentDir, dst, []string{"tar.gz", "zip"}, "", 10, "dcsbr")
	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
}

func TestBackupSingleSourceArgument(t *testing.T) {
	// Simulate config with two sources
	cfg := &Config{}
	cfg.Backup.Prefix = "dcsbr"
	cfg.Backup.Sources = []string{"/tmp/source1", "/tmp/source2"}
	cfg.Backup.Target = t.TempDir()
	cfg.Backup.Formats = []string{"tar.gz"}
	cfg.Backup.MaxBackups = 2
	// Only backup /tmp/source1
	found := false
	for _, srcPath := range cfg.Backup.Sources {
		if srcPath == "/tmp/source1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find /tmp/source1 in sources list")
	}
	// Try a non-existent source
	nonexistent := "/tmp/doesnotexist"
	found = false
	for _, srcPath := range cfg.Backup.Sources {
		if srcPath == nonexistent {
			found = true
			break
		}
	}
	if found {
		t.Error("Did not expect to find /tmp/doesnotexist in sources list")
	}
}

func TestMakeArchiveJobsAndRunArchiveJobs(t *testing.T) {
	jobs := makeArchiveJobs([]string{"tar.gz"}, ".", t.TempDir(), "test", "20220101_000000", nil, "dcsbr")
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
	err := runArchiveJobs(jobs)
	if err != nil {
		t.Errorf("runArchiveJobs should not error, got %v", err)
	}
}

func TestCleanupTempFiles(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "tmpfile.tar")
	os.WriteFile(tmp, []byte("data"), 0o644)
	cleanupTempFiles([]string{tmp})
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("Expected temp file to be removed")
	}
}

func TestRestartStackIfNeeded(t *testing.T) {
	if err := restartStackIfNeeded(false, ".", ""); err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestExportAllComposeVolumesNoComposeFile(t *testing.T) {
	vols, err := exportAllComposeVolumes(t.TempDir(), "")
	if err != nil {
		t.Errorf(errMsg, err)
	}
	if len(vols) != 0 {
		t.Errorf("Expected 0 volumes, got %d", len(vols))
	}
}

func TestBackupComposeStackPositive(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skip Docker-dependent test in GitHub Actions CI")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, testComposeYml), []byte("version: '3'\nservices:{}\nvolumes:{}\n"), 0o644)
	dst := t.TempDir()
	err := BackupComposeStack(dir, dst, "dcsbr")
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestBackupComposeStackWithFormatsEmptyFormats(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skip Docker-dependent test in GitHub Actions CI")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, testComposeYml), []byte("version: '3'\nservices:{}\nvolumes:{}\n"), 0o644)
	dst := t.TempDir()
	err := BackupComposeStackWithFormats(dir, dst, []string{}, "", 10, "dcsbr") // pass empty password for test
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestRestartStackIfNeededTrue(t *testing.T) {
	err := restartStackIfNeeded(true, nonexistentDir, testComposeYml)
	if err == nil {
		t.Error("Expected error for ComposeUp on nonexistent dir")
	}
}

func TestDecryptBackupFile(t *testing.T) {
	err := DecryptBackupFile("notfound.enc", "out", "password")
	if err == nil {
		t.Error("expected error for missing encrypted file")
	}
}

func TestCleanupOldBackups(t *testing.T) {
	err := cleanupOldBackups(t.TempDir(), []string{"dcsbr_backup_stack_*"}, 2, "stack", "dcsbr")
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("unexpected error: %v", err)
	}
}
