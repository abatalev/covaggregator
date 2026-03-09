package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/abatalev/covaggregator/internal/collector"
	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/instance"
	"github.com/abatalev/covaggregator/internal/jacoco"
	"github.com/abatalev/covaggregator/internal/nexus"
	"github.com/abatalev/covaggregator/internal/publisher"
	"github.com/abatalev/covaggregator/internal/storage"
)

var (
	version string
	commit  string
	date    string
)

func printVersion() {
	v := version
	if v == "" {
		v = "dev"
	}
	fmt.Printf("JaCoCo Coverage Aggregator %s (commit: %s, date: %s)\n", v, commit, date)
}

func main() {
	showVersion := flag.Bool("version", false, "Show version information")
	configPath := flag.String("config", "", "Path to configuration YAML file")
	httpAddr := flag.String("http", ":8080", "HTTP address for publisher server (empty to disable)")
	storageRoot := flag.String("storage", "./data", "Root directory for storage")

	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	if *configPath == "" {
		log.Fatal("--config flag is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded configuration with %d service(s)", len(cfg.Services))

	// Check for JaCoCo CLI
	cliPath := jacoco.FindJacocoCLI()
	if cliPath == "" {
		log.Fatal("JaCoCo CLI (jacococli.jar) not found")
	}

	store := storage.NewFSStorage(*storageRoot)
	rep := jacoco.NewReporter(store, cliPath)

	instanceStore := instance.NewStore(store.Root())
	instanceStatuses := instance.NewStatusManager()

	// Миграция инстансов из конфига в хранилище при старте
	migrateInstancesFromConfig(cfg, instanceStore)

	// Запуск HTTP‑сервера, если указан адрес
	if *httpAddr != "" {
		pub := publisher.New(cfg, store, instanceStore, instanceStatuses, version)
		router, err := pub.Router()
		if err != nil {
			log.Fatalf("Failed to create publisher router: %v", err)
		}
		server := &http.Server{
			Addr:         *httpAddr,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		}
		go func() {
			log.Printf("Starting publisher on %s", server.Addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Publisher server error: %v", err)
			}
		}()
	}

	// Запуск сборщика
	col := jacoco.NewCollector(store, cliPath)
	runner := collector.NewRunner(col, rep, store, cfg, instanceStore, instanceStatuses, 5, cliPath)
	go runner.Start()

	for _, svc := range cfg.Services {
		log.Printf("Processing service %q (%s)", svc.ID, svc.Name)
		for _, ver := range svc.Versions {
			log.Printf("  Version %s", ver.Version)

			sv := svc.ServiceVersion(cfg, ver.Version)
			artifactID := svc.GetArtifactID()
			repo := svc.Repository

			// Ensure directories exist
			if err := store.EnsureDirs(sv); err != nil {
				log.Printf("    ERROR: failed to create directories: %v", err)
				continue
			}

			// Handle sources: local path OR download from Nexus
			if ver.SourcePath != "" {
				// Copy from local path
				if _, err := os.Stat(ver.SourcePath); err == nil {
					log.Printf("    Copying sources from %s", ver.SourcePath)
					if err := store.CopyLocalSources(sv, ver.SourcePath); err != nil {
						log.Printf("      WARNING: failed to copy sources: %v", err)
					} else {
						log.Printf("      Sources copied successfully")
					}
				} else {
					log.Printf("    Source path %s does not exist or is inaccessible: %v", ver.SourcePath, err)
				}
			} else if svc.SourcesURLPattern != "" {
				// Download from Nexus if sources not already present
				if !store.HasSources(sv) {
					url := nexus.SourcesURL(svc.SourcesURLPattern, sv, artifactID, repo)
					log.Printf("    Downloading sources from %s", url)
					if err := store.DownloadAndExtract(sv, "sources", url); err != nil {
						log.Printf("      WARNING: failed to download sources: %v", err)
					} else {
						log.Printf("      Sources downloaded successfully")
					}
				} else {
					log.Printf("    Sources already present, skipping download")
				}
			}

			// Handle classes: local path OR download from Nexus
			if ver.ClassPath != "" {
				// Copy from local path
				if _, err := os.Stat(ver.ClassPath); err == nil {
					log.Printf("    Copying classes from %s", ver.ClassPath)
					if err := store.CopyLocalClasses(sv, ver.ClassPath); err != nil {
						log.Printf("      WARNING: failed to copy classes: %v", err)
					} else {
						log.Printf("      Classes copied successfully")
					}
				} else {
					log.Printf("    Class path %s does not exist or is inaccessible: %v", ver.ClassPath, err)
				}
			} else if svc.SourcesURLPattern != "" {
				// Download classes from Nexus if not already present
				if !store.HasClasses(sv) {
					url := nexus.ClassesURL(svc.SourcesURLPattern, sv, artifactID, repo)
					log.Printf("    Downloading classes from %s", url)
					if err := store.DownloadAndExtract(sv, "classes", url); err != nil {
						log.Printf("      WARNING: failed to download classes: %v", err)
					} else {
						log.Printf("      Classes downloaded successfully")
					}
				} else {
					log.Printf("    Classes already present, skipping download")
				}
			}

			// Try to find and copy baseline jacoco.exec
			if ver.ClassPath != "" {
				// Look for jacoco.exec in the target directory (same level as classes)
				classDir := ver.ClassPath
				// If classPath points to a directory like .../target/classes, go up one level to target
				targetDir := filepath.Dir(classDir)
				execPath := filepath.Join(targetDir, "jacoco.exec")
				if _, err := os.Stat(execPath); err == nil {
					log.Printf("    Found baseline jacoco.exec at %s", execPath)
					data, err := os.ReadFile(execPath)
					if err != nil {
						log.Printf("      WARNING: failed to read jacoco.exec: %v", err)
					} else {
						if err := store.SaveExec(sv, "unit-jacoco", data); err != nil {
							log.Printf("      WARNING: failed to save baseline exec: %v", err)
						} else {
							log.Printf("      Baseline exec saved successfully")
						}
					}
				} else {
					log.Printf("    No baseline jacoco.exec found at %s", execPath)
				}
			}

			// Generate full report: try to merge unit + runtime if runtime exists
			generateAllReportsAtInit(store, sv, rep, cliPath)

			// Verify presence
			hasSources := store.HasSources(sv)
			hasClasses := store.HasClasses(sv)
			log.Printf("    Has sources: %v, Has classes: %v", hasSources, hasClasses)
		}
	}

	log.Println("Initialization completed")

	// Бесконечное ожидание (или graceful shutdown)
	select {}
}

// generateAllReportsAtInit генерирует отчеты при инициализации.
func generateAllReportsAtInit(store *storage.FSStorage, sv config.ServiceVersion, rep jacoco.Reporter, cliPath string) {
	unitExecPath := store.GetRepoPath(sv, "unit-jacoco", "jacoco.exec")
	runtimeExecPath := store.GetRepoPath(sv, "runtime-jacoco", "jacoco.exec")
	fullExecPath := store.GetRepoPath(sv, "full-jacoco", "jacoco.exec")

	// Merge: MergeExecFiles filters non-existing files
	if err := jacoco.MergeExecFiles(cliPath, []string{unitExecPath, runtimeExecPath}, fullExecPath); err != nil {
		log.Printf("    WARNING: failed to create full exec: %v", err)
		return
	}

	// Generate reports for unit, runtime and full
	for _, reportType := range []string{"unit-jacoco", "runtime-jacoco", "full-jacoco"} {
		_ = rep.GenerateHTMLReport(sv, reportType)
	}
}

func migrateInstancesFromConfig(cfg *config.Config, instanceStore *instance.Store) {
	for _, svc := range cfg.Services {
		for _, inst := range svc.Instances {
			existing := instanceStore.GetByService(svc.ID)
			found := false
			for _, e := range existing {
				if e.Host == inst.Host && e.Port == inst.Port {
					found = true
					break
				}
			}
			if !found {
				_, err := instanceStore.Add(svc.ID, inst.Host, inst.Port, inst.Version)
				if err != nil {
					log.Printf("  WARNING: failed to migrate instance %s:%d: %v", inst.Host, inst.Port, err)
				} else {
					log.Printf("  Migrated instance %s:%d from config to store", inst.Host, inst.Port)
				}
			}
		}
	}
}
