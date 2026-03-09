package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/storage"
)

// loadExampleThingsWithPaths — адаптированная функция для тестирования, которая принимает корень example.
func loadExampleThingsWithPaths(store storage.Storage, exampleRoot string) error {
	serviceID := "things"
	version := "1.0.0"
	sv := config.ServiceVersion{Service: serviceID, Version: version}

	if err := store.EnsureDirs(sv); err != nil {
		return err
	}

	srcPath := filepath.Join(exampleRoot, "src")
	if err := store.CopyLocalSources(sv, srcPath); err != nil {
		// В тесте мы не используем slog, просто возвращаем ошибку
		return err
	}

	classPath := filepath.Join(exampleRoot, "build/service/target/classes")
	if err := store.CopyLocalClasses(sv, classPath); err != nil {
		return err
	}

	baselinePath := filepath.Join(exampleRoot, "build/service/target/site/jacoco-report/jacoco.exec")
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		// Если файла нет — это не ошибка, просто пропускаем
		return nil
	}
	if err := store.SaveBaselineExec(sv, data); err != nil {
		return err
	}

	return nil
}

func TestLoadExampleThings(t *testing.T) {
	// Создаём временную директорию для хранилища
	tmpDir := t.TempDir()
	store := storage.NewFSStorage(tmpDir)

	sv := config.ServiceVersion{Service: "things", Version: "1.0.0"}

	// Подготавливаем фиктивные данные example/things во временной директории
	exampleRoot := filepath.Join(tmpDir, "example/things")
	// Создаём структуру каталогов и файлов, имитирующую example/things
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "src/main/java/com/example"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(exampleRoot, "src/main/java/com/example/Hello.java"), []byte("public class Hello {}"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "build/service/target/classes/com/example"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(exampleRoot, "build/service/target/classes/com/example/Hello.class"), []byte("fake class bytes"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "build/service/target/site/jacoco-report"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(exampleRoot, "build/service/target/site/jacoco-report/jacoco.exec"), []byte("fake exec data"), 0644))

	// Вызываем тестируемую функцию
	err := loadExampleThingsWithPaths(store, exampleRoot)
	require.NoError(t, err)

	// Проверяем, что директории созданы
	assert.DirExists(t, filepath.Join(tmpDir, "things/1.0.0/sources"))
	assert.DirExists(t, filepath.Join(tmpDir, "things/1.0.0/classes"))
	assert.DirExists(t, filepath.Join(tmpDir, "things/1.0.0/unit-jacoco"))

	// Проверяем, что файлы скопированы
	assert.FileExists(t, filepath.Join(tmpDir, "things/1.0.0/sources/main/java/com/example/Hello.java"))
	assert.FileExists(t, filepath.Join(tmpDir, "things/1.0.0/classes/com/example/Hello.class"))
	assert.FileExists(t, filepath.Join(tmpDir, "things/1.0.0/unit-jacoco/jacoco.exec"))

	// Проверяем содержимое baseline‑файла
	data, err := os.ReadFile(filepath.Join(tmpDir, "things/1.0.0/unit-jacoco/jacoco.exec"))
	require.NoError(t, err)
	assert.Equal(t, []byte("fake exec data"), data)

	// Проверяем методы HasSources и HasClasses
	assert.True(t, store.HasSources(sv))
	assert.True(t, store.HasClasses(sv))
}

func TestLoadExampleThings_NoBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewFSStorage(tmpDir)

	exampleRoot := filepath.Join(tmpDir, "example/things")
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "src"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "build/service/target/classes"), 0755))
	// baseline файл отсутствует

	err := loadExampleThingsWithPaths(store, exampleRoot)
	require.NoError(t, err)

	// Проверяем, что baseline не создан
	baselinePath := filepath.Join(tmpDir, "things/1.0.0/unit-jacoco/jacoco.exec")
	_, err = os.Stat(baselinePath)
	assert.True(t, os.IsNotExist(err), "baseline file should not exist")
}

func TestLoadExampleThings_MissingSources(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewFSStorage(tmpDir)

	exampleRoot := filepath.Join(tmpDir, "example/things")
	// Не создаём src, только классы
	require.NoError(t, os.MkdirAll(filepath.Join(exampleRoot, "build/service/target/classes"), 0755))

	err := loadExampleThingsWithPaths(store, exampleRoot)
	// CopyLocalSources вернёт ошибку, потому что srcPath не существует
	require.Error(t, err)
	// Проверяем, что директории всё равно созданы (EnsureDirs отработал)
	assert.DirExists(t, filepath.Join(tmpDir, "things/1.0.0/sources"))
}
