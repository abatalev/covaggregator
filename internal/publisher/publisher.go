package publisher

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/coverage"
	"github.com/abatalev/covaggregator/internal/instance"
	"github.com/abatalev/covaggregator/internal/nexus"
	"github.com/abatalev/covaggregator/internal/publisher/frontend"
	"github.com/abatalev/covaggregator/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AddInstanceRequest struct {
	ServiceID string `json:"service_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Version   string `json:"version"`
}

// Publisher представляет компонент публикатора с доступом к конфигурации и хранилищу.
type Publisher struct {
	cfg              *config.Config
	st               storage.Storage
	instanceStore    *instance.Store
	instanceStatuses *instance.StatusManager
	version          string
}

// New создаёт новый экземпляр Publisher.
func New(cfg *config.Config, st storage.Storage, instanceStore *instance.Store, instanceStatuses *instance.StatusManager, version string) *Publisher {
	return &Publisher{
		cfg:              cfg,
		st:               st,
		instanceStore:    instanceStore,
		instanceStatuses: instanceStatuses,
		version:          version,
	}
}

// Router возвращает настроенный роутер Chi.
func (p *Publisher) Router() (*chi.Mux, error) {
	r := chi.NewRouter()

	// Базовые middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/health"))

	// Маршруты для отчётов
	p.setupReportRoutes(r)

	// API маршруты
	r.Route("/api", func(r chi.Router) {
		r.Get("/version", p.handleVersion)
		r.Get("/status/table", p.handleStatusTable)
		r.Post("/instances", p.handleAddInstance)
		r.Get("/instances", p.handleListInstances)
		r.Get("/instances/status", p.handleInstanceStatuses)
		r.Get("/nexus/status", p.handleNexusStatus)
	})

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	// Статические файлы фронтенда (SPA)
	if err := p.serveStatic(r); err != nil {
		return nil, err
	}

	return r, nil
}

// serveStatic настраивает раздачу статических файлов из embedded frontend.
func (p *Publisher) serveStatic(r *chi.Mux) error {
	subFS, err := fs.Sub(frontend.Assets, "assets")
	if err != nil {
		return err
	}

	fileServer := http.FileServer(http.FS(subFS))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Use GET /* ", r.URL)
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}

		path := r.URL.Path

		_, err := subFS.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})

	return nil
}

// setupReportRoutes добавляет маршруты для раздачи HTML‑отчётов.
func (p *Publisher) setupReportRoutes(r chi.Router) {
	// Для каждого сервиса и версии из конфигурации монтируем маршруты
	// Используем Chi param
	r.Route("/reports/{service}/{version}", func(r chi.Router) {
		// full отчёт
		r.Get("/full/*", func(w http.ResponseWriter, r *http.Request) {
			log.Println("Use GET /reports/{service}/{version}/full/* ", r.URL)
			service := chi.URLParam(r, "service")
			version := chi.URLParam(r, "version")
			sv := config.ServiceVersion{Service: service, Version: version}
			reportDir := p.st.GetRepoPath(sv, "full-jacoco", "html-report")
			log.Printf("Serving full report from %s", reportDir)
			fs := http.FileServer(http.Dir(reportDir))
			http.StripPrefix("/reports/"+service+"/"+version+"/full", fs).ServeHTTP(w, r)
		})
		// unit отчёт
		r.Get("/unit/*", func(w http.ResponseWriter, r *http.Request) {
			log.Println("Use GET /reports/{service}/{version}/unit/* ", r.URL)
			service := chi.URLParam(r, "service")
			version := chi.URLParam(r, "version")
			sv := config.ServiceVersion{Service: service, Version: version}
			reportDir := p.st.GetRepoPath(sv, "unit-jacoco", "html-report")
			log.Printf("Serving unit report from %s", reportDir)
			fs := http.FileServer(http.Dir(reportDir))
			http.StripPrefix("/reports/"+service+"/"+version+"/unit", fs).ServeHTTP(w, r)
		})
		// runtime отчёт
		r.Get("/runtime/*", func(w http.ResponseWriter, r *http.Request) {
			log.Println("Use GET /reports/{service}/{version}/runtime/* ", r.URL)
			service := chi.URLParam(r, "service")
			version := chi.URLParam(r, "version")
			sv := config.ServiceVersion{Service: service, Version: version}
			reportDir := p.st.GetRepoPath(sv, "runtime-jacoco", "html-report")
			log.Printf("Serving runtime report from %s", reportDir)
			fs := http.FileServer(http.Dir(reportDir))
			http.StripPrefix("/reports/"+service+"/"+version+"/runtime", fs).ServeHTTP(w, r)
		})
	})
}

// StatusRow представляет одну строку таблицы статусов.
type StatusRow struct {
	Service            string        `json:"service"`
	Version            string        `json:"version"`
	HasSources         bool          `json:"hasSources"`
	HasClasses         bool          `json:"hasClasses"`
	HasUnitCoverage    bool          `json:"hasUnitCoverage"`
	UnitCoverage       *CoverageData `json:"unitCoverage,omitempty"`
	UnitReportURL      string        `json:"unitReportUrl,omitempty"`
	HasRuntimeCoverage bool          `json:"hasRuntimeCoverage"`
	RuntimeCoverage    *CoverageData `json:"runtimeCoverage,omitempty"`
	RuntimeReportURL   string        `json:"runtimeReportUrl,omitempty"`
	HasFullCoverage    bool          `json:"hasFullCoverage"`
	FullCoverage       *CoverageData `json:"fullCoverage,omitempty"`
	FullReportURL      string        `json:"fullReportUrl,omitempty"`
	LastUpdated        string        `json:"lastUpdated"`
}

type CoverageData struct {
	Instruction float64 `json:"instruction,omitempty"`
	Line        float64 `json:"line,omitempty"`
	Branch      float64 `json:"branch,omitempty"`
}

// loadCoverage загружает coverage.json для указанного типа отчёта.
func (p *Publisher) loadCoverage(service, version, reportType string) *CoverageData {
	sv := config.ServiceVersion{Service: service, Version: version}
	reportDir := p.st.GetRepoPath(sv, reportType+"-jacoco", "html-report")
	coveragePath := filepath.Join(reportDir, "coverage.json")

	cov, err := coverage.LoadCoverageJSON(coveragePath)
	if err != nil {
		return nil
	}

	data := &CoverageData{}
	if cov.Instruction != nil {
		data.Instruction = cov.Instruction.Percent
	}
	if cov.Line != nil {
		data.Line = cov.Line.Percent
	}
	if cov.Branch != nil {
		data.Branch = cov.Branch.Percent
	}
	return data
}

// handleVersion returns the version information.
func (p *Publisher) handleVersion(w http.ResponseWriter, _ *http.Request) {
	v := p.version
	if v == "" {
		v = "dev"
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"version": v}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleStatusTable возвращает актуальные данные таблицы статусов.
func (p *Publisher) handleStatusTable(w http.ResponseWriter, _ *http.Request) {
	var rows []StatusRow

	for _, svc := range p.cfg.Services {
		for _, ver := range svc.Versions {
			sv := config.ServiceVersion{Service: svc.ID, Version: ver.Version}
			hasSources := p.st.HasSources(sv)
			hasClasses := p.st.HasClasses(sv)
			hasUnit := p.st.HasReport(sv, "unit-jacoco")
			hasRuntime := p.st.HasReport(sv, "runtime-jacoco")
			hasFull := p.st.HasReport(sv, "full-jacoco")
			lastUpdated := p.st.LastUpdated(sv)

			// Загружаем данные о покрытии
			var unitCov, runtimeCov, fullCov *CoverageData
			if hasUnit {
				unitCov = p.loadCoverage(svc.ID, ver.Version, "unit")
			}
			if hasRuntime {
				runtimeCov = p.loadCoverage(svc.ID, ver.Version, "runtime")
			}
			if hasFull {
				fullCov = p.loadCoverage(svc.ID, ver.Version, "full")
			}

			// Формируем URL отчётов (если отчёт есть)
			unitURL := ""
			if hasUnit {
				unitURL = "/reports/" + svc.ID + "/" + ver.Version + "/unit/"
			}
			runtimeURL := ""
			if hasRuntime {
				runtimeURL = "/reports/" + svc.ID + "/" + ver.Version + "/runtime/"
			}
			fullURL := ""
			if hasFull {
				fullURL = "/reports/" + svc.ID + "/" + ver.Version + "/full/"
			}

			rows = append(rows, StatusRow{
				Service:            svc.ID,
				Version:            ver.Version,
				HasSources:         hasSources,
				HasClasses:         hasClasses,
				HasUnitCoverage:    hasUnit,
				UnitCoverage:       unitCov,
				UnitReportURL:      unitURL,
				HasRuntimeCoverage: hasRuntime,
				RuntimeCoverage:    runtimeCov,
				RuntimeReportURL:   runtimeURL,
				HasFullCoverage:    hasFull,
				FullCoverage:       fullCov,
				FullReportURL:      fullURL,
				LastUpdated:        lastUpdated,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}

func (p *Publisher) handleAddInstance(w http.ResponseWriter, r *http.Request) {
	var req AddInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ServiceID == "" {
		http.Error(w, "service_id is required", http.StatusBadRequest)
		return
	}
	if req.Host == "" {
		http.Error(w, "host is required", http.StatusBadRequest)
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}

	inst, err := p.instanceStore.Add(req.ServiceID, req.Host, req.Port, req.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-prepare storage for the new instance version (download from Nexus if needed)
	go p.prepareStorageForInstance(req.ServiceID, req.Version)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(inst); err != nil {
		log.Printf("Failed to encode instance: %v", err)
	}
}

// prepareStorageForInstance downloads sources and classes from Nexus if not already present.
func (p *Publisher) prepareStorageForInstance(serviceID, version string) {
	if version == "" {
		log.Printf("  WARNING: no version provided for service %s, skipping storage preparation", serviceID)
		return
	}

	// Find service config
	var svc config.Service
	for _, s := range p.cfg.Services {
		if s.ID == serviceID {
			svc = s
			break
		}
	}
	if svc.ID == "" {
		log.Printf("  WARNING: service %s not found in config, skipping storage preparation", serviceID)
		return
	}

	sv := p.cfg.ServiceVersion(serviceID, version)
	artifactID := svc.GetArtifactID()
	repo := svc.Repository

	// Ensure directories exist
	if err := p.st.EnsureDirs(sv); err != nil {
		log.Printf("  WARNING: failed to create directories for %s:%s: %v", serviceID, version, err)
		return
	}

	// Download sources if configured and not present
	if svc.SourcesURLPattern != "" && !p.st.HasSources(sv) {
		url := nexus.SourcesURL(svc.SourcesURLPattern, sv, artifactID, repo)
		log.Printf("  Downloading sources from %s", url)
		if err := p.st.DownloadAndExtract(sv, "sources", url); err != nil {
			log.Printf("  WARNING: failed to download sources for %s:%s: %v", serviceID, version, err)
		} else {
			log.Printf("  Sources downloaded successfully for %s:%s", serviceID, version)
		}
	}

	// Download classes if configured and not present
	if svc.SourcesURLPattern != "" && !p.st.HasClasses(sv) {
		url := nexus.ClassesURL(svc.SourcesURLPattern, sv, artifactID, repo)
		log.Printf("  Downloading classes from %s", url)
		if err := p.st.DownloadAndExtract(sv, "classes", url); err != nil {
			log.Printf("  WARNING: failed to download classes for %s:%s: %v", serviceID, version, err)
		} else {
			log.Printf("  Classes downloaded successfully for %s:%s", serviceID, version)
		}
	}
}

func (p *Publisher) handleListInstances(w http.ResponseWriter, _ *http.Request) {
	instances := p.instanceStore.List()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("Failed to encode instances: %v", err)
	}
}

func (p *Publisher) handleInstanceStatuses(w http.ResponseWriter, _ *http.Request) {
	statuses := p.instanceStatuses.ListStatuses()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		log.Printf("Failed to encode statuses: %v", err)
	}
}

type NexusStatus struct {
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	Reachable bool   `json:"reachable"`
}

func (p *Publisher) handleNexusStatus(w http.ResponseWriter, _ *http.Request) {
	var nexusURL string
	for _, svc := range p.cfg.Services {
		if svc.SourcesURLPattern != "" {
			nexusURL = extractNexusURL(svc.SourcesURLPattern)
			break
		}
	}

	status := NexusStatus{
		URL:       nexusURL,
		Enabled:   nexusURL != "",
		Reachable: false,
	}

	if nexusURL != "" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(nexusURL)
		if err == nil {
			defer resp.Body.Close()
			status.Reachable = resp.StatusCode < 500
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Failed to encode nexus status: %v", err)
	}
}

func extractNexusURL(pattern string) string {
	idx := strings.Index(pattern, "://")
	if idx == -1 {
		return ""
	}
	endIdx := strings.Index(pattern[idx+3:], "/")
	if endIdx == -1 {
		return ""
	}
	return pattern[:idx+3+endIdx]
}
