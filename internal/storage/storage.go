package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/iohelper"
)

// Storage определяет интерфейс для работы с файловым хранилищем данных покрытия.
type Storage interface {
	// EnsureDirs создаёт все необходимые поддиректории для указанной версии сервиса.
	EnsureDirs(sv config.ServiceVersion) error

	// CopyLocalSources рекурсивно копирует исходные тексты из локального пути в директорию sources хранилища.
	CopyLocalSources(sv config.ServiceVersion, srcPath string) error

	// CopyLocalClasses рекурсивно копирует скомпилированные классы из локального пути в директорию classes хранилища.
	CopyLocalClasses(sv config.ServiceVersion, classPath string) error

	// SaveBaselineExec сохраняет данные baseline‑покрытия (файл .exec) в соответствующую директорию.
	SaveBaselineExec(sv config.ServiceVersion, data []byte) error

	// SaveExec сохраняет данные .exec‑файла для указанного типа отчёта.
	SaveExec(sv config.ServiceVersion, repoType string, data []byte) error

	// GetExecPath возвращает путь к файлу .exec для указанного типа отчёта.
	GetExecPath(sv config.ServiceVersion, repoType string) string

	// SaveRuntimeExec сохраняет данные runtime‑покрытия (файл .exec) для конкретного инстанса.
	// Файл сохраняется в runtime-jacoco/hosts/<host>_<port>/<timestamp>.exec.
	SaveRuntimeExec(sv config.ServiceVersion, host string, port int, data []byte) error

	// HasSources возвращает true, если для указанной версии сервиса есть хотя бы один исходный файл.
	HasSources(sv config.ServiceVersion) bool

	// HasClasses возвращает true, если для указанной версии сервиса есть хотя бы один класс‑файл.
	HasClasses(sv config.ServiceVersion) bool

	// HasReport возвращает true, если существует HTML‑отчёт для указанного типа покрытия.
	HasReport(sv config.ServiceVersion, repoType string) bool

	// LastUpdated возвращает временную метку последнего обновления данных для версии сервиса.
	// Формат: RFC3339 (например, "2026-03-10T07:30:00Z").
	// Если метаданные отсутствуют, возвращает пустую строку.
	LastUpdated(sv config.ServiceVersion) string

	// GetRepoPath возвращает путь внутри директории версии сервиса.
	GetRepoPath(sv config.ServiceVersion, args ...string) string

	// DownloadAndExtract загружает артефакт по URL и распаковывает в указанную поддиректорию.
	// subdir - поддиректория внутри версии сервиса (например, "sources" или "classes").
	DownloadAndExtract(sv config.ServiceVersion, subdir, url string) error

	// Root возвращает корневую директорию хранилища.
	Root() string
}

// FSStorage реализует Storage на основе файловой системы.
type FSStorage struct {
	root string
}

// NewFSStorage создаёт новый экземпляр FSStorage с указанным корневым каталогом.
func NewFSStorage(root string) *FSStorage {
	return &FSStorage{root: root}
}

// serviceVersionDir возвращает абсолютный путь к директории версии сервиса.
func (s *FSStorage) serviceVersionDir(sv config.ServiceVersion) string {
	return filepath.Join(s.root, sv.Service, sv.Version)
}

// EnsureDirs создаёт все поддиректории согласно структуре хранилища.
func (s *FSStorage) EnsureDirs(sv config.ServiceVersion) error {
	base := s.serviceVersionDir(sv)
	dirs := []string{
		filepath.Join(base, "sources"),
		filepath.Join(base, "classes"),
		filepath.Join(base, "unit-jacoco"),
		filepath.Join(base, "unit-jacoco", "html-report"),
		filepath.Join(base, "runtime-jacoco", "hosts"),
		filepath.Join(base, "runtime-jacoco", "html-report"),
		filepath.Join(base, "full-jacoco"),
		filepath.Join(base, "full-jacoco", "html-report"),
		filepath.Join(base, "meta"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// copyDir рекурсивно копирует содержимое директории src в dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
}

// copyFile копирует файл из src в dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CopyLocalSources реализует Storage.CopyLocalSources.
func (s *FSStorage) CopyLocalSources(sv config.ServiceVersion, srcPath string) error {
	dst := filepath.Join(s.serviceVersionDir(sv), "sources")
	return copyDir(srcPath, dst)
}

// CopyLocalClasses реализует Storage.CopyLocalClasses.
func (s *FSStorage) CopyLocalClasses(sv config.ServiceVersion, classPath string) error {
	dst := filepath.Join(s.serviceVersionDir(sv), "classes")
	return copyDir(classPath, dst)
}

// SaveBaselineExec сохраняет данные .exec‑файла в unit-jacoco/jacoco.exec.
func (s *FSStorage) SaveBaselineExec(sv config.ServiceVersion, data []byte) error {
	return s.SaveExec(sv, "unit-jacoco", data)
}

// SaveExec сохраняет данные .exec‑файла для указанного типа отчёта.
func (s *FSStorage) SaveExec(sv config.ServiceVersion, repoType string, data []byte) error {
	path := s.GetExecPath(sv, repoType)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return iohelper.AtomicWrite(path, data)
}

// GetExecPath возвращает путь к файлу .exec для указанного типа отчёта.
func (s *FSStorage) GetExecPath(sv config.ServiceVersion, repoType string) string {
	return s.GetRepoPath(sv, repoType, "jacoco.exec")
}

// GetBaselineExec возвращает данные baseline .exec‑файла.
func (s *FSStorage) GetBaselineExec(sv config.ServiceVersion) ([]byte, error) {
	return os.ReadFile(s.GetExecPath(sv, "unit-jacoco"))
}

// SaveRuntimeExec сохраняет данные runtime‑покрытия (файл .exec) для конкретного инстанса.
// Файл сохраняется в runtime-jacoco/hosts/<host>_<port>/jacoco.exec (перезаписывает предыдущий).
// Использует атомарную запись через временный файл с переименованием.
func (s *FSStorage) SaveRuntimeExec(sv config.ServiceVersion, host string, port int, data []byte) error {
	hostDir := filepath.Join(s.serviceVersionDir(sv), "runtime-jacoco", "hosts", host+"_"+strconv.Itoa(port))
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(hostDir, "jacoco.exec")

	if err := iohelper.AtomicWrite(path, data); err != nil {
		return err
	}
	return s.updateLastUpdated(sv)
}

// updateLastUpdated обновляет временную метку последнего обновления.
func (s *FSStorage) updateLastUpdated(sv config.ServiceVersion) error {
	metaDir := filepath.Join(s.serviceVersionDir(sv), "meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}
	metaFile := filepath.Join(metaDir, "last_updated.txt")
	data := []byte(time.Now().Format(time.RFC3339))
	return os.WriteFile(metaFile, data, 0644)
}

// HasSources проверяет наличие исходных файлов.
func (s *FSStorage) HasSources(sv config.ServiceVersion) bool {
	sourcesDir := filepath.Join(s.serviceVersionDir(sv), "sources")
	found := false
	_ = filepath.Walk(sourcesDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".java" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// HasClasses проверяет наличие класс‑файлов.
func (s *FSStorage) HasClasses(sv config.ServiceVersion) bool {
	classesDir := filepath.Join(s.serviceVersionDir(sv), "classes")
	found := false
	_ = filepath.Walk(classesDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".class" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// HasReport проверяет наличие HTML‑отчёта для указанного типа покрытия.
func (s *FSStorage) HasReport(sv config.ServiceVersion, repoType string) bool {
	reportIndex := filepath.Join(s.serviceVersionDir(sv), repoType, "html-report", "index.html")
	_, err := os.Stat(reportIndex)
	return err == nil
}

// LastUpdated возвращает временную метку последнего обновления.
func (s *FSStorage) LastUpdated(sv config.ServiceVersion) string {
	metaFile := filepath.Join(s.serviceVersionDir(sv), "meta", "last_updated.txt")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return ""
	}
	// Убираем пробельные символы в конце и возвращаем
	return strings.TrimSpace(string(data))
}

// GetRepoPath возвращает путь внутри директории версии сервиса.
func (s *FSStorage) GetRepoPath(sv config.ServiceVersion, args ...string) string {
	return filepath.Join(append([]string{s.serviceVersionDir(sv)}, args...)...)
}

// DownloadAndExtract загружает артефакт по URL и распаковывает в указанную поддиректорию.
func (s *FSStorage) DownloadAndExtract(sv config.ServiceVersion, subdir, url string) error {
	destDir := s.GetRepoPath(sv, subdir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", destDir, err)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "download-"+time.Now().Format("20060102150405")+".jar")

	if err := downloadFile(tmpFile, url); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer os.Remove(tmpFile)

	if err := extractJar(tmpFile, destDir); err != nil {
		return fmt.Errorf("extract %s: %w", tmpFile, err)
	}

	return nil
}

func downloadFile(filepath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractJar(jarPath, destDir string) error {
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Skip directories and non-java files optionally
		if f.FileInfo().IsDir() {
			continue
		}

		outPath := filepath.Join(destDir, f.Name)

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Root возвращает корневую директорию хранилища.
func (s *FSStorage) Root() string {
	return s.root
}
