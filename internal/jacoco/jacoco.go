package jacoco

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/coverage"
	"github.com/abatalev/covaggregator/internal/iohelper"
	"github.com/abatalev/covaggregator/internal/metrics"
	"github.com/abatalev/covaggregator/internal/storage"
)

func FindJacocoCLI() string {
	possiblePaths := []string{
		"tools/jacococli.jar",
		filepath.Join(".", "tools", "jacococli.jar"),
		"jacococli.jar",
		"/usr/local/lib/jacococli.jar",
	}
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

type Collector interface {
	Collect(serviceID string, instance config.Instance, versionDetection *config.VersionDetection) ([]byte, config.ServiceVersion, error)
}

type collectorImpl struct {
	st      storage.Storage
	cliPath string
}

func NewCollector(st storage.Storage, cliPath string) Collector {
	return &collectorImpl{
		st:      st,
		cliPath: cliPath,
	}
}

func (c *collectorImpl) Collect(serviceID string, instance config.Instance, versionDetection *config.VersionDetection) ([]byte, config.ServiceVersion, error) {
	version := instance.Version
	if version == "" && versionDetection != nil && versionDetection.Enabled {
		detected, err := detectVersion(instance, versionDetection)
		if err != nil {
			log.Printf("Failed to detect version for %s:%d: %v", instance.Host, instance.Port, err)
			return nil, config.ServiceVersion{}, fmt.Errorf("version detection failed: %w", err)
		}
		version = detected
		log.Printf("Detected version %s for %s:%d", version, instance.Host, instance.Port)
	}
	if version == "" {
		return nil, config.ServiceVersion{}, fmt.Errorf("version is required for instance %s:%d", instance.Host, instance.Port)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("dump_%s_%s_%d_%d.exec", serviceID, instance.Host, instance.Port, time.Now().Unix()))
	defer os.Remove(tmpFile)

	cmd := exec.Command("java", "-jar", c.cliPath, "dump",
		"--address", instance.Host,
		"--port", fmt.Sprintf("%d", instance.Port),
		"--destfile", tmpFile,
		"--reset",
	)
	log.Printf("Executing jacococli dump for %s:%d", instance.Host, instance.Port)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, config.ServiceVersion{}, fmt.Errorf("jacococli dump failed: %v, output: %s", err, output)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, config.ServiceVersion{}, fmt.Errorf("failed to read dump file: %w", err)
	}

	log.Printf("Successfully collected dump from %s:%d, size %d bytes", instance.Host, instance.Port, len(data))
	sv := config.ServiceVersion{Service: serviceID, Version: version}
	return data, sv, nil
}

func detectVersion(instance config.Instance, detection *config.VersionDetection) (string, error) {
	url := fmt.Sprintf("http://%s:%d%s", instance.Host, instance.Port, detection.Endpoint)
	ctx, cancel := context.WithTimeout(context.Background(), detection.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("empty version response")
	}
	return version, nil
}

type Reporter interface {
	GenerateHTMLReport(sv config.ServiceVersion, reportType string) error
}

type reporterImpl struct {
	st      storage.Storage
	cliPath string
}

func NewReporter(st storage.Storage, cliPath string) Reporter {
	return &reporterImpl{
		st:      st,
		cliPath: cliPath,
	}
}

func (r *reporterImpl) GenerateHTMLReport(sv config.ServiceVersion, reportType string) error {
	execPath := r.st.GetRepoPath(sv, reportType, "jacoco.exec")
	htmlDir := r.st.GetRepoPath(sv, reportType, "html-report")
	sourcesDir := r.st.GetRepoPath(sv, "sources")
	classesDir := r.st.GetRepoPath(sv, "classes")

	if _, err := os.Stat(execPath); err != nil {
		log.Printf("    Exec file not found, skipping report generation: %v", err)
		return nil
	}

	xmlPath := filepath.Join(htmlDir, "coverage.xml")

	cmd := exec.Command("java", "-jar", r.cliPath, "report", execPath,
		"--classfiles", classesDir,
		"--sourcefiles", sourcesDir,
		"--html", htmlDir,
		"--xml", xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("    Failed to generate HTML report: %v, output: %s", err, output)
		return createPlaceholder(htmlDir)
	}

	log.Printf("    HTML report generated in %s", htmlDir)

	if cov, err := coverage.ParseXML(xmlPath); err != nil {
		log.Printf("    WARNING: failed to parse coverage.xml: %v", err)
	} else if err := coverage.SaveCoverageJSON(cov, htmlDir); err != nil {
		log.Printf("    WARNING: failed to save coverage.json: %v", err)
	} else {
		log.Printf("    Saved coverage.json")
		metrics.UpdateCoverageMetrics(sv.Service, sv.Version, reportType, cov)
	}

	return nil
}

func createPlaceholder(htmlDir string) error {
	placeholder := filepath.Join(htmlDir, "index.html")
	content := `<html><body><h1>JaCoCo Coverage Report</h1><p>Report generation requires jacococli.jar. Please install JaCoCo CLI.</p></body></html>`
	if err := os.WriteFile(placeholder, []byte(content), 0644); err != nil {
		log.Printf("      WARNING: failed to write placeholder: %v", err)
		return err
	}
	log.Printf("      Placeholder HTML created at %s", placeholder)
	return nil
}

func MergeExecFiles(cliPath string, execFiles []string, outputPath string) error {
	// Filter out non-existing files
	var existingFiles []string
	for _, f := range execFiles {
		if _, err := os.Stat(f); err == nil {
			existingFiles = append(existingFiles, f)
		}
	}
	execFiles = existingFiles

	if len(execFiles) == 0 {
		return fmt.Errorf("no exec files to merge")
	}

	if len(execFiles) == 1 {
		return iohelper.CopyFile(execFiles[0], outputPath)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(outputPath), "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{"-jar", cliPath, "merge"}
	args = append(args, execFiles...)
	args = append(args, "--destfile", tmpPath)

	cmd := exec.Command("java", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jacococli merge: %v, output: %s", err, output)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
