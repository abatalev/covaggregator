package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantErr  bool
		validate func(t *testing.T, cfg *Config)
	}{
		{
			name:    "valid config",
			path:    "testdata/config.yaml",
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Services, 2)

				svcA := cfg.Services[0]
				assert.Equal(t, "service-a", svcA.ID)
				assert.Equal(t, "Service A", svcA.Name)
				assert.Equal(t, "60s", svcA.PollInterval)
				assert.Len(t, svcA.Versions, 2)
				assert.Len(t, svcA.Instances, 2)
				assert.NotNil(t, svcA.VersionDetection)
				assert.True(t, svcA.VersionDetection.Enabled)
				assert.Equal(t, "/version", svcA.VersionDetection.Endpoint)
				assert.Equal(t, 5*time.Second, svcA.VersionDetection.Timeout)

				svcB := cfg.Services[1]
				assert.Equal(t, "service-b", svcB.ID)
				assert.Equal(t, "Service B", svcB.Name)
				assert.Equal(t, "30s", svcB.PollInterval)
				assert.Nil(t, svcB.VersionDetection)
			},
		},
		{
			name:    "non-existent file",
			path:    "testdata/nonexistent.yaml",
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			path:    "testdata/invalid.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestServiceValidation(t *testing.T) {
	tests := []struct {
		name     string
		service  Service
		wantID   string
		wantName string
	}{
		{
			name: "simple service",
			service: Service{
				ID:   "test",
				Name: "Test Service",
			},
			wantID:   "test",
			wantName: "Test Service",
		},
		{
			name: "service with versions",
			service: Service{
				ID:   "svc",
				Name: "Service",
				Versions: []Version{
					{Version: "1.0.0", SourcePath: "/src", ClassPath: "/cls"},
				},
			},
			wantID:   "svc",
			wantName: "Service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantID, tt.service.ID)
			assert.Equal(t, tt.wantName, tt.service.Name)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errText string
	}{
		{
			name:    "empty services",
			config:  Config{Services: []Service{}},
			wantErr: true,
			errText: "at least one service must be defined",
		},
		{
			name: "missing service ID",
			config: Config{Services: []Service{
				{Name: "Test", PollInterval: "10s", Versions: []Version{{Version: "1.0.0"}}},
			}},
			wantErr: true,
			errText: "service ID is required",
		},
		{
			name: "duplicate service ID",
			config: Config{Services: []Service{
				{ID: "svc1", Name: "Test1", PollInterval: "10s", Versions: []Version{{Version: "1.0.0", SourcePath: "/src1", ClassPath: "/cls1"}}},
				{ID: "svc1", Name: "Test2", PollInterval: "20s", Versions: []Version{{Version: "2.0.0", SourcePath: "/src2", ClassPath: "/cls2"}}},
			}},
			wantErr: true,
			errText: "duplicate service ID: svc1",
		},
		{
			name: "missing service name",
			config: Config{Services: []Service{
				{ID: "svc1", PollInterval: "10s", Versions: []Version{{Version: "1.0.0"}}},
			}},
			wantErr: true,
			errText: "service svc1: name is required",
		},
		{
			name: "invalid poll interval",
			config: Config{Services: []Service{
				{ID: "svc1", Name: "Test", PollInterval: "invalid", Versions: []Version{{Version: "1.0.0"}}},
			}},
			wantErr: true,
			errText: "service svc1: invalid poll_interval",
		},
		{
			name: "no versions",
			config: Config{Services: []Service{
				{ID: "svc1", Name: "Test", PollInterval: "10s", Versions: []Version{}},
			}},
			wantErr: true,
			errText: "service svc1: at least one version must be defined",
		},
		{
			name: "version missing source and class paths",
			config: Config{Services: []Service{
				{ID: "svc1", Name: "Test", PollInterval: "10s", Versions: []Version{{Version: "1.0.0"}}},
			}},
			wantErr: true,
			errText: "service svc1 version 1.0.0: either source_path/class_path or sources_url_pattern must be provided",
		},
		{
			name: "instance invalid port",
			config: Config{Services: []Service{
				{
					ID: "svc1", Name: "Test", PollInterval: "10s",
					Versions:  []Version{{Version: "1.0.0", SourcePath: "/src"}},
					Instances: []Instance{{Host: "localhost", Port: 0}},
				},
			}},
			wantErr: true,
			errText: "service svc1: instance port 0 is invalid",
		},
		{
			name: "valid config",
			config: Config{Services: []Service{
				{
					ID: "svc1", Name: "Test", PollInterval: "10s",
					Versions:  []Version{{Version: "1.0.0", SourcePath: "/src", ClassPath: "/cls"}},
					Instances: []Instance{{Host: "localhost", Port: 8080}},
				},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errText != "" {
					assert.Contains(t, err.Error(), tt.errText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
