package iohelper

import (
	"os"
	"path/filepath"
)

func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return AtomicWrite(dst, data)
}
