package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServiceVersion represents a service and version pair.
type ServiceVersion struct {
	Service string // service ID
	Version string
	GroupID string // resolved from config
}

// Config returns ServiceVersion with GroupID resolved from global config and service.
func (c *Config) ServiceVersion(serviceID, version string) ServiceVersion {
	for _, svc := range c.Services {
		if svc.ID == serviceID {
			return ServiceVersion{
				Service: serviceID,
				Version: version,
				GroupID: c.GetGroupID(svc.GroupID),
			}
		}
	}
	return ServiceVersion{Service: serviceID, Version: version}
}

// Config represents the application configuration.
type Config struct {
	GroupID  string    `yaml:"group_id,omitempty"` // global groupId for all services
	Services []Service `yaml:"services"`
}

// GetGroupID returns the groupId for a service, falling back to global config.
func (c *Config) GetGroupID(serviceGroupID string) string {
	if serviceGroupID != "" {
		return serviceGroupID
	}
	return c.GroupID
}

// Service represents a single service with its instances and versions.
type Service struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	GroupID           string            `yaml:"group_id,omitempty"`    // overrides global GroupID
	ArtifactID        string            `yaml:"artifact_id,omitempty"` // defaults to ID if empty
	Repository        string            `yaml:"repository,omitempty"`  // nexus repository
	SourcesURLPattern string            `yaml:"sources_url_pattern"`   // with {{suffix}}
	Versions          []Version         `yaml:"versions"`
	Instances         []Instance        `yaml:"instances"`
	PollInterval      string            `yaml:"poll_interval"`
	VersionDetection  *VersionDetection `yaml:"version_detection,omitempty"`
}

// GetArtifactID returns the artifactId for a service, defaulting to service ID.
func (s *Service) GetArtifactID() string {
	if s.ArtifactID != "" {
		return s.ArtifactID
	}
	return s.ID
}

// ServiceVersion returns ServiceVersion with GroupID resolved from config.
func (s *Service) ServiceVersion(cfg *Config, version string) ServiceVersion {
	return ServiceVersion{
		Service: s.ID,
		Version: version,
		GroupID: cfg.GetGroupID(s.GroupID),
	}
}

// Version holds source and class paths for a specific version.
type Version struct {
	Version    string `yaml:"version"`
	SourcePath string `yaml:"source_path"`
	ClassPath  string `yaml:"class_path"`
}

// Instance represents a JVM instance to poll.
type Instance struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Version string `yaml:"version,omitempty"`
}

// VersionDetection holds configuration for automatic version detection.
type VersionDetection struct {
	Enabled  bool          `yaml:"enabled"`
	Endpoint string        `yaml:"endpoint"`
	Timeout  time.Duration `yaml:"timeout"`
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the configuration for logical errors and missing required fields.
func (c *Config) Validate() error {
	if len(c.Services) == 0 {
		return errors.New("at least one service must be defined")
	}
	seenIDs := make(map[string]bool)
	for _, svc := range c.Services {
		if svc.ID == "" {
			return errors.New("service ID is required")
		}
		if seenIDs[svc.ID] {
			return fmt.Errorf("duplicate service ID: %s", svc.ID)
		}
		seenIDs[svc.ID] = true

		if svc.Name == "" {
			return fmt.Errorf("service %s: name is required", svc.ID)
		}
		if _, err := time.ParseDuration(svc.PollInterval); err != nil {
			return fmt.Errorf("service %s: invalid poll_interval %q: %v", svc.ID, svc.PollInterval, err)
		}
		if len(svc.Versions) == 0 {
			return fmt.Errorf("service %s: at least one version must be defined", svc.ID)
		}
		for _, ver := range svc.Versions {
			if ver.Version == "" {
				return fmt.Errorf("service %s: version string is required", svc.ID)
			}
			// Allow either local paths OR sources_url_pattern
			hasLocalPath := ver.SourcePath != "" || ver.ClassPath != ""
			hasURLPattern := svc.SourcesURLPattern != ""
			if !hasLocalPath && !hasURLPattern {
				return fmt.Errorf("service %s version %s: either source_path/class_path or sources_url_pattern must be provided", svc.ID, ver.Version)
			}
		}
		for _, inst := range svc.Instances {
			if inst.Host == "" {
				return fmt.Errorf("service %s: instance host is required", svc.ID)
			}
			if inst.Port <= 0 || inst.Port > 65535 {
				return fmt.Errorf("service %s: instance port %d is invalid", svc.ID, inst.Port)
			}
		}
		if svc.VersionDetection != nil && svc.VersionDetection.Enabled {
			if svc.VersionDetection.Endpoint == "" {
				return fmt.Errorf("service %s: version_detection.endpoint is required when enabled", svc.ID)
			}
			if svc.VersionDetection.Timeout <= 0 {
				return fmt.Errorf("service %s: version_detection.timeout must be positive", svc.ID)
			}
		}
	}
	return nil
}
