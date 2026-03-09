package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Instance struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type PersistedInstance struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	instances map[string]PersistedInstance
	mu        sync.RWMutex
	filePath  string
}

func NewStore(storageRoot string) *Store {
	filePath := filepath.Join(storageRoot, "instances.json")
	s := &Store{
		instances: make(map[string]PersistedInstance),
		filePath:  filePath,
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to load instances: %v\n", err)
		}
		return
	}
	var instances []PersistedInstance
	if err := json.Unmarshal(data, &instances); err != nil {
		fmt.Printf("Failed to parse instances.json: %v\n", err)
		return
	}
	for _, inst := range instances {
		s.instances[inst.ID] = inst
	}
}

func (s *Store) save() {
	var instances []PersistedInstance
	s.mu.RLock()
	for _, inst := range s.instances {
		instances = append(instances, inst)
	}
	s.mu.RUnlock()

	if len(instances) == 0 {
		if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Failed to remove instances file: %v\n", err)
		}
		return
	}

	data, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal instances: %v\n", err)
		return
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		fmt.Printf("Failed to save instances: %v\n", err)
	}
}

func (s *Store) Add(serviceID, host string, port int, version string) (PersistedInstance, error) {
	if host == "" {
		return PersistedInstance{}, fmt.Errorf("host is required")
	}
	if port <= 0 || port > 65535 {
		return PersistedInstance{}, fmt.Errorf("invalid port %d", port)
	}

	id := fmt.Sprintf("%s_%s_%d", serviceID, host, port)
	inst := PersistedInstance{
		ID:        id,
		ServiceID: serviceID,
		Host:      host,
		Port:      port,
		Version:   version,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.instances[id] = inst
	s.mu.Unlock()

	s.save()
	return inst, nil
}

func (s *Store) Get(id string) (PersistedInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.instances[id]
	return inst, ok
}

func (s *Store) GetByService(serviceID string) []PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []PersistedInstance
	for _, inst := range s.instances {
		if inst.ServiceID == serviceID {
			result = append(result, inst)
		}
	}
	return result
}

func (s *Store) List() []PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PersistedInstance, 0, len(s.instances))
	for _, inst := range s.instances {
		result = append(result, inst)
	}
	return result
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.instances[id]; ok {
		delete(s.instances, id)
		go s.save()
		return true
	}
	return false
}

func (s *Store) GetAllInstances() map[string]PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]PersistedInstance, len(s.instances))
	for k, v := range s.instances {
		result[k] = v
	}
	return result
}
