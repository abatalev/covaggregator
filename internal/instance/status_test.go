package instance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatusManager_UpdateStatus(t *testing.T) {
	sm := NewStatusManager()

	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "success", "")

	status := sm.GetStatus("inst1")
	assert.NotNil(t, status)
	assert.Equal(t, "inst1", status.InstanceID)
	assert.Equal(t, "localhost", status.Host)
	assert.Equal(t, 8080, status.Port)
	assert.Equal(t, "svc1", status.ServiceID)
	assert.Equal(t, "1.0.0", status.Version)
	assert.Equal(t, "success", status.Status)
	assert.NotZero(t, status.LastPoll)
}

func TestStatusManager_GetStatus_NotFound(t *testing.T) {
	sm := NewStatusManager()

	status := sm.GetStatus("nonexistent")
	assert.Nil(t, status)
}

func TestStatusManager_GetAllStatuses(t *testing.T) {
	sm := NewStatusManager()

	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "success", "")
	sm.UpdateStatus("inst2", "localhost", 8081, "svc1", "1.0.0", "error", "timeout")

	all := sm.GetAllStatuses()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "inst1")
	assert.Contains(t, all, "inst2")
}

func TestStatusManager_ListStatuses(t *testing.T) {
	sm := NewStatusManager()

	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "success", "")
	sm.UpdateStatus("inst2", "localhost", 8081, "svc1", "1.0.0", "error", "timeout")

	list := sm.ListStatuses()
	assert.Len(t, list, 2)

	var ids []string
	for _, s := range list {
		ids = append(ids, s.InstanceID)
	}
	assert.Contains(t, ids, "inst1")
	assert.Contains(t, ids, "inst2")
}

func TestStatusManager_UpdateStatus_Multiple(t *testing.T) {
	sm := NewStatusManager()

	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "polling", "")
	time.Sleep(10 * time.Millisecond)
	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "success", "")

	status := sm.GetStatus("inst1")
	assert.Equal(t, "success", status.Status)

	list := sm.ListStatuses()
	assert.Len(t, list, 1)
	assert.Equal(t, "success", list[0].Status)
}

func TestStatusManager_LastError(t *testing.T) {
	sm := NewStatusManager()

	sm.UpdateStatus("inst1", "localhost", 8080, "svc1", "1.0.0", "error", "connection refused")

	status := sm.GetStatus("inst1")
	assert.Equal(t, "error", status.Status)
	assert.Equal(t, "connection refused", status.LastError)
}

func TestStatusManager_Empty(t *testing.T) {
	sm := NewStatusManager()

	all := sm.GetAllStatuses()
	assert.Empty(t, all)

	list := sm.ListStatuses()
	assert.Empty(t, list)
}
