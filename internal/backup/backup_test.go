package backup

import (
	"os"
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
	err := BackupComposeStackWithFormats(nonexistentDir, dst, []string{"tar.gz", "zip"})
	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
}

func TestMakeArchiveJobsAndRunArchiveJobs(t *testing.T) {
	jobs := makeArchiveJobs([]string{"tar.gz", "zip"}, ".", t.TempDir(), "test", "20220101_000000", nil)
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
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
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, testComposeYml), []byte("version: '3'\nservices:{}\nvolumes:{}\n"), 0o644)
	dst := t.TempDir()
	err := BackupComposeStack(dir, dst)
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestBackupComposeStackWithFormatsEmptyFormats(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, testComposeYml), []byte("version: '3'\nservices:{}\nvolumes:{}\n"), 0o644)
	dst := t.TempDir()
	err := BackupComposeStackWithFormats(dir, dst, []string{})
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

func TestExportAllComposeVolumesErrorOnList(t *testing.T) {
	_, err := exportAllComposeVolumes(nonexistentDir, testComposeYml)
	if err == nil {
		t.Error("Expected error for ListComposeVolumes on nonexistent dir")
	}
}
