package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	hdr, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return err
	}
	hdr.Name = relPath
	if err := tarw.WriteHeader(hdr); err != nil {
		return err
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
	if info.IsDir() {
		return nil
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
	cmd := exec.Command("docker", "run", "--rm", "--name", containerName, "-v", volume+":"+mountPath+":ro", "-v", os.TempDir()+":/backup", "alpine", "sh", "-c", tarCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return tarPath, nil
}
