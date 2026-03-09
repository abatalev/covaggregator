package instance

import (
	"sync"
	"time"
)

type Status struct {
	InstanceID string    `json:"instance_id"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	ServiceID  string    `json:"service_id"`
	Version    string    `json:"version"`
	Status     string    `json:"status"`
	LastPoll   time.Time `json:"last_poll,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

type StatusManager struct {
	statuses map[string]*Status
	mu       sync.RWMutex
}

func NewStatusManager() *StatusManager {
	return &StatusManager{
		statuses: make(map[string]*Status),
	}
}

func (sm *StatusManager) UpdateStatus(instanceID, host string, port int, serviceID, version, status string, lastError string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.statuses[instanceID] = &Status{
		InstanceID: instanceID,
		Host:       host,
		Port:       port,
		ServiceID:  serviceID,
		Version:    version,
		Status:     status,
		LastPoll:   time.Now(),
		LastError:  lastError,
	}
}

func (sm *StatusManager) GetStatus(instanceID string) *Status {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.statuses[instanceID]
}

func (sm *StatusManager) GetAllStatuses() map[string]*Status {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]*Status)
	for k, v := range sm.statuses {
		if v != nil {
			result[k] = v
		}
	}
	return result
}

func (sm *StatusManager) ListStatuses() []Status {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]Status, 0, len(sm.statuses))
	for _, v := range sm.statuses {
		if v != nil {
			result = append(result, *v)
		}
	}
	return result
}
