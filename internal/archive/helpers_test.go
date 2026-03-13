package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	errMsg  = "Expected no error, got %v"
	fileTxt = "file.txt"
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
	defer f.Close()
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
	defer f.Close()
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

func TestTarGzFolderWithVolumesIgnoresUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are not supported in this test on Windows")
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	socketPath := filepath.Join(src, "test.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer listener.Close()

	dst := t.TempDir()
	tarPath := filepath.Join(dst, "test.tar.gz")
	if err := TarGzFolderWithVolumes(src, tarPath, nil); err != nil {
		t.Fatalf("TarGzFolderWithVolumes should ignore unix sockets: %v", err)
	}

	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("Failed to open tar.gz: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to open gzip: %v", err)
	}
	defer gz.Close()

	tarReader := tar.NewReader(gz)
	foundFile := false
	foundSocket := false
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		if hdr.Name == "file1.txt" {
			foundFile = true
		}
		if hdr.Name == "test.sock" {
			foundSocket = true
		}
	}
	if !foundFile {
		t.Fatalf("file1.txt not found in tar archive")
	}
	if foundSocket {
		t.Fatalf("test.sock should not be in the tar archive")
	}
}

func TestZipFolderWithVolumesIgnoresUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are not supported in this test on Windows")
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file2.txt"), []byte("zip world"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	socketPath := filepath.Join(src, "test.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer listener.Close()

	dst := t.TempDir()
	zipPath := filepath.Join(dst, "test.zip")
	if err := ZipFolderWithVolumes(src, zipPath, nil); err != nil {
		t.Fatalf("ZipFolderWithVolumes should ignore unix sockets: %v", err)
	}

	zipr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip reader: %v", err)
	}
	defer zipr.Close()

	foundFile := false
	foundSocket := false
	for _, zf := range zipr.File {
		if zf.Name == "file2.txt" {
			foundFile = true
		}
		if zf.Name == "test.sock" {
			foundSocket = true
		}
	}
	if !foundFile {
		t.Fatalf("file2.txt not found in zip archive")
	}
	if foundSocket {
		t.Fatalf("test.sock should not be in the zip archive")
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
	err := AddFileToTarWithGitIgnore(tmp, f, file, nil, tarw)
	tarw.Close()
	out.Close()
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
	err := AddFileToTar(tmp, f, file, nil, tarw)
	tarw.Close()
	out.Close()
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
	err := AddFileToZipWithGitIgnore(tmp, f, file, nil, zipw)
	zipw.Close()
	out.Close()
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
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skip Docker volume export test in GitHub Actions CI")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}
	tarPath, err := ExportDockerVolumeTar("nonexistent", "/volume")
	defer os.Remove(tarPath)
	if err != nil {
		return // acceptable: engine/runtime can reject the command
	}
	stat, statErr := os.Stat(tarPath)
	if statErr != nil {
		t.Fatalf("expected tar output file when export succeeds, got stat error: %v", statErr)
	}
	if stat.Size() <= 0 {
		t.Fatalf("expected non-empty tar output for successful export, got size=%d", stat.Size())
	}
}

func TestEncryptFileAndDecryptFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.txt")
	dst := filepath.Join(tmpDir, "test.txt.enc")
	dec := filepath.Join(tmpDir, "test.txt.dec")
	password := "thisisaverysecurepassword"
	content := []byte("secret data")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}
	if err := EncryptFile(src, dst, password); err != nil {
		t.Errorf("EncryptFile failed: %v", err)
	}
	if err := DecryptFile(dst, dec, password); err != nil {
		t.Errorf("DecryptFile failed: %v", err)
	}
	decContent, err := os.ReadFile(dec)
	if err != nil {
		t.Fatalf("failed to read decrypted file: %v", err)
	}
	if string(decContent) != string(content) {
		t.Errorf("decrypted content mismatch: got %q, want %q", decContent, content)
	}
}

func TestCheckDirReadable(t *testing.T) {
	dir := t.TempDir()
	if err := CheckDirReadable(dir); err != nil {
		t.Errorf("CheckDirReadable failed: %v", err)
	}
}

func TestCheckDirReadableIgnoresUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are not supported in this test on Windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create unix socket: %v", err)
	}
	defer listener.Close()

	if err := CheckDirReadable(dir); err != nil {
		t.Fatalf("CheckDirReadable should ignore unix sockets: %v", err)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	file := filepath.Join(src, fileTxt)
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := CopyDir(src, dst); err != nil {
		t.Errorf("CopyDir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, fileTxt)); err != nil {
		t.Errorf("file not copied: %v", err)
	}
}

func TestExtractTarGzZipTar(t *testing.T) {
	tmpDir := t.TempDir()
	// Just check that the functions return error for non-existent files
	if err := ExtractTarGz("nofile.tar.gz", tmpDir); err == nil {
		t.Error("expected error for ExtractTarGz with non-existent file")
	}
	if err := ExtractZip("nofile.zip", tmpDir); err == nil {
		t.Error("expected error for ExtractZip with non-existent file")
	}
	if err := ExtractTar("nofile.tar", tmpDir); err == nil {
		t.Error("expected error for ExtractTar with non-existent file")
	}
}
