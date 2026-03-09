package instance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Add(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	inst, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "svc1", inst.ServiceID)
	assert.Equal(t, "localhost", inst.Host)
	assert.Equal(t, 8080, inst.Port)
	assert.Equal(t, "1.0.0", inst.Version)

	list := store.List()
	assert.Len(t, list, 1)
}

func TestStore_Add_InvalidHost(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "", 8080, "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestStore_Add_InvalidPort(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "localhost", 0, "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")

	_, err = store.Add("svc1", "localhost", 70000, "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")
}

func TestStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	inst, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)

	found, ok := store.Get(inst.ID)
	assert.True(t, ok)
	assert.Equal(t, inst, found)

	_, ok = store.Get("nonexistent")
	assert.False(t, ok)
}

func TestStore_GetByService(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)
	_, err = store.Add("svc1", "localhost", 8081, "1.0.0")
	require.NoError(t, err)
	_, err = store.Add("svc2", "localhost", 8080, "2.0.0")
	require.NoError(t, err)

	instances := store.GetByService("svc1")
	assert.Len(t, instances, 2)

	instances = store.GetByService("svc2")
	assert.Len(t, instances, 1)

	instances = store.GetByService("svc3")
	assert.Len(t, instances, 0)
}

func TestStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "host1", 8080, "1.0.0")
	require.NoError(t, err)
	_, err = store.Add("svc2", "host2", 8080, "2.0.0")
	require.NoError(t, err)

	list := store.List()
	assert.Len(t, list, 2)
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	inst, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)

	deleted := store.Delete(inst.ID)
	assert.True(t, deleted)

	list := store.List()
	assert.Len(t, list, 0)
}

func TestStore_Delete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)
	deleted := store.Delete("svc1_nonexistent")
	assert.False(t, deleted)
}

func TestStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	store1 := NewStore(tmpDir)
	_, err := store1.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)

	store2 := NewStore(tmpDir)
	time.Sleep(20 * time.Millisecond)
	list := store2.List()
	assert.Len(t, list, 1)
	assert.Equal(t, "svc1", list[0].ServiceID)
}

func TestStore_GetAllInstances(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "host1", 8080, "1.0.0")
	require.NoError(t, err)
	_, err = store.Add("svc1", "host1", 8081, "1.0.0")
	require.NoError(t, err)
	_, err = store.Add("svc2", "host2", 8080, "2.0.0")
	require.NoError(t, err)
}

func TestStore_DuplicateInstance_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)

	inst2, err := store.Add("svc1", "localhost", 8080, "2.0.0")
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", inst2.Version)

	time.Sleep(20 * time.Millisecond)

	list := store.List()
	assert.Len(t, list, 1)
	assert.Equal(t, "2.0.0", list[0].Version)
}

func TestStore_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Add("svc1", "localhost", 8080, "1.0.0")
	require.NoError(t, err)

	filePath := filepath.Join(tmpDir, "instances.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)
}
