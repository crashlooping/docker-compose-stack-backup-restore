package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	fileTxt = "file.txt"
	errMsg  = "Expected no error, got %v"
)

func TestArchiveHelpersSanity(t *testing.T) {
	// This is a placeholder test. Add real tests for archive helpers here.
}

func TestTarGzFolderWithVolumesArchivesFiles(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello world"), 0o644)
	os.Mkdir(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "should_ignore.txt"), []byte("ignore me"), 0o644)
	dst := t.TempDir()
	tarPath := filepath.Join(dst, "test.tar.gz")
	if err := TarGzFolderWithVolumes(src, tarPath, nil); err != nil {
		t.Fatalf("TarGzFolderWithVolumes failed: %v", err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("Failed to open tar.gz: %v", err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to open gzip: %v", err)
	}
	defer gz.Close()
	buf := make([]byte, 512)
	_, err = gz.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read tar.gz: %v", err)
	}
}

func TestZipFolderWithVolumesArchivesFiles(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file2.txt"), []byte("zip world"), 0o644)
	os.Mkdir(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "should_ignore.txt"), []byte("ignore me"), 0o644)
	dst := t.TempDir()
	zipPath := filepath.Join(dst, "test.zip")
	if err := ZipFolderWithVolumes(src, zipPath, nil); err != nil {
		t.Fatalf("ZipFolderWithVolumes failed: %v", err)
	}
	f, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("Failed to stat zip: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatalf("Zip file is empty")
	}
	zipr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip reader: %v", err)
	}
	defer zipr.Close()
	found := false
	for _, zf := range zipr.File {
		if zf.Name == "file2.txt" {
			found = true
		}
		if zf.Name == ".git/should_ignore.txt" {
			t.Fatalf(".git/should_ignore.txt should not be in the zip archive")
		}
	}
	if !found {
		t.Fatalf("file2.txt not found in zip archive")
	}
}

func TestAddFileToTarWithGitIgnoreErrors(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, fileTxt)
	os.WriteFile(f, []byte("x"), 0o644)
	file, _ := os.Stat(f)
	tarFile := filepath.Join(tmp, "out.tar")
	out, _ := os.Create(tarFile)
	tarw := tar.NewWriter(out)
	defer tarw.Close()
	err := AddFileToTarWithGitIgnore(tmp, f, file, nil, tarw)
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestAddFileToTarErrors(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, fileTxt)
	os.WriteFile(f, []byte("x"), 0o644)
	file, _ := os.Stat(f)
	tarFile := filepath.Join(tmp, "out.tar")
	out, _ := os.Create(tarFile)
	tarw := tar.NewWriter(out)
	defer tarw.Close()
	err := AddFileToTar(tmp, f, file, nil, tarw)
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestAddVolumeTarToTarGzErrors(t *testing.T) {
	err := AddVolumeTarToTarGz("/nonexistent.tar", tar.NewWriter(io.Discard))
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestAddFileToZipWithGitIgnoreErrors(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, fileTxt)
	os.WriteFile(f, []byte("x"), 0o644)
	file, _ := os.Stat(f)
	zipFile := filepath.Join(tmp, "out.zip")
	out, _ := os.Create(zipFile)
	zipw := zip.NewWriter(out)
	defer zipw.Close()
	err := AddFileToZipWithGitIgnore(tmp, f, file, nil, zipw)
	if err != nil {
		t.Errorf(errMsg, err)
	}
}

func TestAddVolumeTarToZipErrors(t *testing.T) {
	err := AddVolumeTarToZip("/nonexistent.tar", zip.NewWriter(io.Discard))
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestExportDockerVolumeTarSkipIfNoDocker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}
	tarPath, err := ExportDockerVolumeTar("nonexistent", "/volume")
	if err == nil {
		t.Errorf("Expected error for nonexistent volume, got tarPath=%v", tarPath)
	}
}
