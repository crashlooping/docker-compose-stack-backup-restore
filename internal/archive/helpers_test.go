package archive

import (
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
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
