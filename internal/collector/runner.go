package collector

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/abatalev/covaggregator/internal/config"
	"github.com/abatalev/covaggregator/internal/instance"
	"github.com/abatalev/covaggregator/internal/jacoco"
	"github.com/abatalev/covaggregator/internal/storage"
)

// Runner управляет периодическим сбором данных покрытия со всех инстансов.
type Runner struct {
	collector        jacoco.Collector
	reporter         jacoco.Reporter
	store            storage.Storage
	cfg              *config.Config
	instanceStore    *instance.Store
	instanceStatuses *instance.StatusManager
	workers          int
	stop             chan struct{}
	wg               sync.WaitGroup
	jacocoCLI        string
}

// NewRunner создаёт новый Runner.
func NewRunner(collector jacoco.Collector, reporter jacoco.Reporter, store storage.Storage, cfg *config.Config, instanceStore *instance.Store, instanceStatuses *instance.StatusManager, workers int, cliPath string) *Runner {
	return &Runner{
		collector:        collector,
		reporter:         reporter,
		store:            store,
		cfg:              cfg,
		instanceStore:    instanceStore,
		instanceStatuses: instanceStatuses,
		workers:          workers,
		stop:             make(chan struct{}),
		jacocoCLI:        cliPath,
	}
}

// Start запускает runner для каждого сервиса в отдельной горутине.
func (r *Runner) Start() {
	for _, svc := range r.cfg.Services {
		r.wg.Add(1)
		go r.runService(svc)
	}
	log.Printf("Runner started for %d service(s)", len(r.cfg.Services))
}

// Stop останавливает все горутины runner'а.
func (r *Runner) Stop() {
	close(r.stop)
	r.wg.Wait()
	log.Println("Runner stopped")
}

// runService выполняет периодический опрос для одного сервиса.
func (r *Runner) runService(svc config.Service) {
	defer r.wg.Done()

	pollInterval, err := time.ParseDuration(svc.PollInterval)
	if err != nil {
		log.Printf("Service %s: invalid poll interval %q: %v", svc.ID, svc.PollInterval, err)
		return
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Первый опрос сразу
	r.pollService(svc)

	for {
		select {
		case <-ticker.C:
			r.pollService(svc)
		case <-r.stop:
			return
		}
	}
}

// pollService опрашивает все инстансы сервиса конкурентно.
func (r *Runner) pollService(svc config.Service) {
	start := time.Now()
	log.Printf("Polling service %s (%s) started", svc.ID, svc.Name)

	// Получаем инстансы из хранилища
	var allInstances []config.Instance

	if r.instanceStore != nil {
		instances := r.instanceStore.GetByService(svc.ID)
		for _, di := range instances {
			allInstances = append(allInstances, config.Instance{
				Host:    di.Host,
				Port:    di.Port,
				Version: di.Version,
			})
		}
	}

	if len(allInstances) == 0 {
		log.Printf("  Service %s: no instances to poll", svc.ID)
		return
	}

	// Создаём каналы для задач и результатов
	tasks := make(chan config.Instance, len(allInstances))
	results := make(chan error, len(allInstances))

	// Запускаем воркеры
	var wg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go r.worker(svc.ID, svc.VersionDetection, tasks, results, &wg)
	}

	// Отправляем задачи
	for _, inst := range allInstances {
		tasks <- inst
	}
	close(tasks)

	// Ждём завершения воркеров
	wg.Wait()
	close(results)

	// Обрабатываем ошибки
	var errCount, successCount int
	for res := range results {
		if res != nil {
			errCount++
		} else {
			successCount++
		}
	}

	elapsed := time.Since(start)
	if errCount > 0 {
		log.Printf("  Service %s: finished in %v, %d succeeded, %d failed", svc.ID, elapsed, successCount, errCount)
	} else {
		log.Printf("  Service %s: finished in %v, all %d instances succeeded", svc.ID, elapsed, successCount)
	}

	// Генерируем отчёты для каждой версии из конфигурации
	for _, v := range svc.Versions {
		if err := r.generateRuntimeReport(svc.ID, v.Version); err != nil {
			log.Printf("  WARNING: failed to generate runtime report for version %s: %v", v.Version, err)
		}
	}
}

// worker обрабатывает задачи из канала tasks.
func (r *Runner) worker(serviceID string, versionDetection *config.VersionDetection, tasks <-chan config.Instance, results chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for inst := range tasks {
		results <- r.collectInstance(serviceID, inst, versionDetection)
	}
}

// collectInstance выполняет сбор данных для одного инстанса.
func (r *Runner) collectInstance(serviceID string, inst config.Instance, versionDetection *config.VersionDetection) error {
	instanceID := fmt.Sprintf("%s_%s_%d", serviceID, inst.Host, inst.Port)

	// Обновляем статус на "polling"
	if r.instanceStatuses != nil {
		r.instanceStatuses.UpdateStatus(instanceID, inst.Host, inst.Port, serviceID, inst.Version, "polling", "")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Используем канал для получения результата сбора
	done := make(chan struct {
		data    []byte
		version config.ServiceVersion
		err     error
	}, 1)

	go func() {
		data, sv, err := r.collector.Collect(serviceID, inst, versionDetection)
		done <- struct {
			data    []byte
			version config.ServiceVersion
			err     error
		}{data, sv, err}
	}()

	select {
	case <-ctx.Done():
		if r.instanceStatuses != nil {
			r.instanceStatuses.UpdateStatus(instanceID, inst.Host, inst.Port, serviceID, inst.Version, "error", "timeout")
		}
		return fmt.Errorf("timeout collecting from %s:%d", inst.Host, inst.Port)
	case res := <-done:
		if res.err != nil {
			if r.instanceStatuses != nil {
				r.instanceStatuses.UpdateStatus(instanceID, inst.Host, inst.Port, serviceID, inst.Version, "error", res.err.Error())
			}
			return res.err
		}
		// Сохраняем данные с мерджем с предыдущим exec (т.к. используется --reset)
		log.Printf("  Collected %d bytes from %s:%d, version %s", len(res.data), inst.Host, inst.Port, res.version.Version)
		dataToSave, err := r.mergeWithExistingExec(serviceID, res.version.Version, inst.Host, inst.Port, res.data)
		if err != nil {
			log.Printf("  Failed to merge with existing exec: %v, saving new data", err)
			dataToSave = res.data
		}
		if err := r.store.SaveRuntimeExec(res.version, inst.Host, inst.Port, dataToSave); err != nil {
			log.Printf("  Failed to save runtime exec: %v", err)
			if r.instanceStatuses != nil {
				r.instanceStatuses.UpdateStatus(instanceID, inst.Host, inst.Port, serviceID, res.version.Version, "error", fmt.Sprintf("save error: %v", err))
			}
			return fmt.Errorf("save runtime exec: %w", err)
		}
		log.Printf("  Saved runtime exec for %s:%d", inst.Host, inst.Port)

		// Обновляем статус на success
		if r.instanceStatuses != nil {
			r.instanceStatuses.UpdateStatus(instanceID, inst.Host, inst.Port, serviceID, res.version.Version, "success", "")
		}

		return nil
	}
}

// generateRuntimeReport генерирует отчёт для всех инстансов.
func (r *Runner) generateRuntimeReport(serviceID, version string) error {
	sv := config.ServiceVersion{Service: serviceID, Version: version}
	hostsDir := r.store.GetRepoPath(sv, "runtime-jacoco", "hosts")
	mergedFile := r.store.GetRepoPath(sv, "runtime-jacoco", "jacoco.exec")

	var execFiles []string
	err := filepath.Walk(hostsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".exec" {
			execFiles = append(execFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk hosts dir: %w", err)
	}

	if len(execFiles) == 0 {
		log.Printf("    No exec files found")
		return nil
	}

	runtimeJacocoDir := r.store.GetRepoPath(sv, "runtime-jacoco")
	if err := os.MkdirAll(runtimeJacocoDir, 0755); err != nil {
		return fmt.Errorf("create runtime-jacoco dir: %w", err)
	}

	if err := jacoco.MergeExecFiles(r.jacocoCLI, execFiles, mergedFile); err != nil {
		return fmt.Errorf("merge exec files: %w", err)
	}
	log.Printf("    Merged %d exec files to %s", len(execFiles), mergedFile)

	if r.reporter != nil {
		if err := r.reporter.GenerateHTMLReport(config.ServiceVersion{Service: serviceID, Version: version}, "runtime"); err != nil {
			log.Printf("    WARNING: failed to generate report: %v", err)
		}
	}

	if err := r.generateFullReport(serviceID, version); err != nil {
		log.Printf("    WARNING: failed to generate full report: %v", err)
	}

	return nil
}

// generateFullReport объединяет unit и runtime данные и генерирует full отчет.
func (r *Runner) generateFullReport(serviceID, version string) error {
	sv := config.ServiceVersion{Service: serviceID, Version: version}
	unitExecPath := r.store.GetRepoPath(sv, "unit-jacoco", "jacoco.exec")
	runtimeExecPath := r.store.GetRepoPath(sv, "runtime-jacoco", "jacoco.exec")
	fullExecPath := r.store.GetRepoPath(sv, "full-jacoco", "jacoco.exec")

	// MergeExecFiles filters non-existing files
	if err := jacoco.MergeExecFiles(r.jacocoCLI, []string{unitExecPath, runtimeExecPath}, fullExecPath); err != nil {
		return fmt.Errorf("merge unit + runtime: %w", err)
	}
	log.Printf("      Saved full jacoco.exec (unit + runtime)")

	if r.reporter != nil {
		if err := r.reporter.GenerateHTMLReport(config.ServiceVersion{Service: serviceID, Version: version}, "full"); err != nil {
			log.Printf("      WARNING: failed to generate full report: %v", err)
		}
	}

	return nil
}

// mergeWithExistingExec мерджит новый dump с существующим exec файлом для хоста.
// Используется потому что jacococli dump --reset сбрасывает данные на JVM side,
// поэтому нужно накапливать данные локально.
func (r *Runner) mergeWithExistingExec(service, version, host string, port int, newData []byte) ([]byte, error) {
	sv := config.ServiceVersion{Service: service, Version: version}
	hostDir := r.store.GetRepoPath(sv, "runtime-jacoco", "hosts", host+"_"+strconv.Itoa(port))
	execPath := filepath.Join(hostDir, "jacoco.exec")

	existingData, err := os.ReadFile(execPath)
	if err != nil {
		if os.IsNotExist(err) {
			return newData, nil
		}
		return nil, err
	}

	newDataFile := filepath.Join(hostDir, "new-temp.exec")
	if err := os.WriteFile(newDataFile, newData, 0644); err != nil {
		return nil, err
	}
	defer os.Remove(newDataFile)

	mergedFile := filepath.Join(hostDir, "merged-temp.exec")
	if err := jacoco.MergeExecFiles(r.jacocoCLI, []string{execPath, newDataFile}, mergedFile); err != nil {
		os.Remove(mergedFile)
		return nil, err
	}

	mergedData, err := os.ReadFile(mergedFile)
	if err != nil {
		os.Remove(mergedFile)
		return nil, err
	}
	os.Remove(mergedFile)

	log.Printf("    Merged new dump with existing exec (%d + %d = %d bytes)",
		len(existingData), len(newData), len(mergedData))

	return mergedData, nil
}
