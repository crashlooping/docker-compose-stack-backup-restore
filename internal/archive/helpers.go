package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TarGzFolderWithVolumes(srcDir, destFile string, volumeTarballs []string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tarw := tar.NewWriter(gz)
	defer tarw.Close()
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return AddFileToTarWithGitIgnore(srcDir, path, info, err, tarw)
	})
	if err != nil {
		return err
	}
	for _, v := range volumeTarballs {
		if err := AddVolumeTarToTarGz(v, tarw); err != nil {
			return err
		}
	}
	return nil
}

// TarZstFolderWithVolumes creates a .tar.zst archive using Zstandard compression.
func TarZstFolderWithVolumes(srcDir, destFile string, volumeTarballs []string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zstWriter, err := zstd.NewWriter(f, zstd.WithEncoderConcurrency(runtime.GOMAXPROCS(0)))
	if err != nil {
		return err
	}
	defer zstWriter.Close()
	tarw := tar.NewWriter(zstWriter)
	defer tarw.Close()
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return AddFileToTarWithGitIgnore(srcDir, path, info, err, tarw)
	})
	if err != nil {
		return err
	}
	for _, v := range volumeTarballs {
		if err := AddVolumeTarToTarGz(v, tarw); err != nil {
			return err
		}
	}
	return nil
}

func AddFileToTarWithGitIgnore(srcDir, path string, info os.FileInfo, err error, tarw *tar.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	return AddFileToTar(srcDir, path, info, err, tarw)
}

func AddFileToTar(srcDir, path string, info os.FileInfo, err error, tarw *tar.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == "." {
		return nil
	}
	if info.Mode()&(os.ModeSocket|os.ModeNamedPipe|os.ModeDevice|os.ModeCharDevice) != 0 {
		return nil
	}
	hdr, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return err
	}
	hdr.Name = relPath
	if err := tarw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.IsDir() {
		// Always write header for directories, even if empty
		return nil
	}
	if info.Mode().IsRegular() {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarw, file)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddVolumeTarToTarGz(volumeTarPath string, tarw *tar.Writer) error {
	file, err := os.Open(volumeTarPath)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fmt.Printf("Adding volume tarball '%s' to backup (size: %d bytes)\n", volumeTarPath, stat.Size())
	hdr := &tar.Header{
		Name:    filepath.Join("volumes", filepath.Base(volumeTarPath)),
		Size:    stat.Size(),
		Mode:    0o600,
		ModTime: stat.ModTime(),
	}
	if err := tarw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tarw, file)
	return err
}

func ZipFolderWithVolumes(srcDir, destFile string, volumeTarballs []string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zipw := zip.NewWriter(f)
	defer zipw.Close()
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return AddFileToZipWithGitIgnore(srcDir, path, info, err, zipw)
	})
	if err != nil {
		return err
	}
	for _, v := range volumeTarballs {
		if err := AddVolumeTarToZip(v, zipw); err != nil {
			return err
		}
	}
	return nil
}

func AddFileToZipWithGitIgnore(srcDir, path string, info os.FileInfo, err error, zipw *zip.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if info.Mode()&(os.ModeSocket|os.ModeNamedPipe|os.ModeDevice|os.ModeCharDevice) != 0 {
		return nil
	}
	if info.IsDir() {
		// Always create directory entry in zip, even if empty
		_, err := zipw.Create(relPath + "/")
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w, err := zipw.Create(relPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, file)
	return err
}

func AddVolumeTarToZip(volumeTarPath string, zipw *zip.Writer) error {
	file, err := os.Open(volumeTarPath)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fmt.Printf("Adding volume tarball '%s' to zip backup (size: %d bytes)\n", volumeTarPath, stat.Size())
	w, err := zipw.Create(filepath.Join("volumes", filepath.Base(volumeTarPath)))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, file)
	return err
}

func ExportDockerVolumeTar(volume, mountPath string) (string, error) {
	tarPath := filepath.Join(os.TempDir(), volume+".tar")
	containerName := "tmp-vol-backup-" + volume + "-" + fmt.Sprint(time.Now().UnixNano())
	fmt.Printf("Exporting docker volume '%s' (mount path: %s) to tarball. Host path: %s\n", volume, mountPath, tarPath)
	tarCmd := fmt.Sprintf("tar cf /backup/%s.tar -C %s .", volume, mountPath)
	cmd := exec.Command("docker", "run", "--rm", "--name", containerName, "-v", volume+":"+mountPath+":ro", "-v", os.TempDir()+":/backup", "alpine:3", "sh", "-c", tarCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return tarPath, nil
}

// ExtractTarGz extracts a .tar.gz archive to a target directory
// safePath validates that the resolved path stays within the destination directory.
func safePath(dest, entryPath string) (string, error) {
	cleanPath := filepath.Clean(filepath.Join(dest, entryPath))
	cleanDest := filepath.Clean(dest)
	if !strings.HasPrefix(cleanPath, cleanDest+string(os.PathSeparator)) && cleanPath != cleanDest {
		return "", fmt.Errorf("path traversal detected: %s escapes destination %s", entryPath, dest)
	}
	return cleanPath, nil
}

func ExtractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path, err := safePath(dest, hdr.Name)
		if err != nil {
			return err
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		io.Copy(out, tarReader)
		out.Close()
	}
	return nil
}

// ExtractTarZst extracts a .tar.zst archive to a target directory
func ExtractTarZst(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	zstReader, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zstReader.Close()
	tarReader := tar.NewReader(zstReader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path, err := safePath(dest, hdr.Name)
		if err != nil {
			return err
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		io.Copy(out, tarReader)
		out.Close()
	}
	return nil
}

// ExtractZip extracts a .zip archive to a target directory
func ExtractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		path, err := safePath(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		io.Copy(out, in)
		in.Close()
		out.Close()
	}
	return nil
}

// ExtractTar extracts a .tar archive to a target directory
func ExtractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	tarReader := tar.NewReader(f)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path, err := safePath(dest, hdr.Name)
		if err != nil {
			return err
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		io.Copy(out, tarReader)
		out.Close()
	}
	return nil
}

// CopyDir copies a directory recursively
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		tgt := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(tgt, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(tgt, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// CheckDirReadable walks a directory and returns an error if any regular file or directory cannot be read.
func CheckDirReadable(dir string) error {
	var unreadable []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			unreadable = append(unreadable, path+": "+err.Error())
			// Skip further descent into this directory
			if os.IsPermission(err) && info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&(os.ModeSocket|os.ModeNamedPipe|os.ModeDevice|os.ModeCharDevice) != 0 {
			return nil
		}
		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				unreadable = append(unreadable, path+": "+err.Error())
				return nil
			}
			f.Close()
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(unreadable) > 0 {
		return fmt.Errorf("unreadable files or directories detected:\n%s", strings.Join(unreadable, "\n"))
	}
	return nil
}

// EncryptFile encrypts srcPath to dstPath using password (AES-256-GCM). Overwrites dstPath if exists.
func EncryptFile(srcPath, dstPath, password string) error {
	if len(password) < 16 {
		return fmt.Errorf("encryption password must be at least 16 characters long")
	}
	key := sha256.Sum256([]byte(password))
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	if err := os.WriteFile(dstPath, append(nonce, ciphertext...), 0o600); err != nil {
		return err
	}
	return nil
}

// DecryptFile decrypts srcPath to dstPath using password (AES-256-GCM). Returns error if password is wrong.
func DecryptFile(srcPath, dstPath, password string) error {
	key := sha256.Sum256([]byte(password))
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if len(data) < 12 {
		return fmt.Errorf("decryption failed: invalid ciphertext")
	}
	nonce, ciphertext := data[:12], data[12:]
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed: %w (wrong password or corrupted data)", err)
	}
	if err := os.WriteFile(dstPath, plaintext, 0o600); err != nil {
		return err
	}
	return nil
}
