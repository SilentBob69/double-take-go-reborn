package utils

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"double-take-go-reborn/internal/core/processor"
	
	"github.com/shirou/gopsutil/v3/cpu"
	log "github.com/sirupsen/logrus"
)

var (
	lastCPUTime        time.Time
	lastCPUUsage       float64
	cpuUsageMutex      sync.Mutex
	cpuUsageSampleRate = 500 * time.Millisecond
)

// SystemStats enthält aktuelle System- und Anwendungsstatistiken
type SystemStats struct {
	// CPU-Statistiken
	NumCPU       int     `json:"num_cpu"`
	GoRoutines   int     `json:"go_routines"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  uint64  `json:"memory_usage"`
	MemoryAlloc  uint64  `json:"memory_alloc"`
	MemorySys    uint64  `json:"memory_sys"`
	
	// Worker-Pool-Statistiken
	WorkerCount   int `json:"worker_count"`
	ActiveJobs    int `json:"active_jobs"`
	QueueCapacity int `json:"queue_capacity"`
	
	// Zeitstempel
	Timestamp time.Time `json:"timestamp"`
}

// FormatBytes formatiert Bytes in lesbare Einheiten (KB, MB, GB)
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d Bytes", bytes)
	}
}

// GetCPUUsage berechnet die CPU-Auslastung mit gopsutil
func GetCPUUsage() float64 {
	cpuUsageMutex.Lock()
	defer cpuUsageMutex.Unlock()
	
	// Wenn weniger als 500ms seit dem letzten Sampling vergangen sind,
	// den gecachten Wert zurückgeben
	if time.Since(lastCPUTime) < cpuUsageSampleRate && lastCPUTime.Unix() > 0 {
		return lastCPUUsage
	}
	
	// CPU-Auslastung mit gopsutil messen (intervall von 200ms für schnelle Messung)
	percentages, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil {
		log.Warnf("Fehler bei CPU-Auslastungsmessung: %v", err)
		return 0.0
	}
	
	var usage float64
	if len(percentages) > 0 {
		usage = percentages[0] // Gesamtauslastung aller Kerne
	}
	
	// Cache aktualisieren
	lastCPUTime = time.Now()
	lastCPUUsage = usage
	
	return usage
}

// GetSystemStats erfasst aktuelle System- und Anwendungsstatistiken
func GetSystemStats(workerPool *processor.WorkerPool) *SystemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	stats := &SystemStats{
		NumCPU:       runtime.NumCPU(),
		GoRoutines:   runtime.NumGoroutine(),
		CPUUsage:     GetCPUUsage(),
		MemoryAlloc:  memStats.Alloc,
		MemorySys:    memStats.Sys,
		Timestamp:    time.Now(),
	}
	
	// Worker-Pool-Statistiken, falls verfügbar
	if workerPool != nil {
		stats.WorkerCount = workerPool.GetWorkerCount()
		stats.ActiveJobs = workerPool.ActiveJobCount()
		stats.QueueCapacity = workerPool.GetQueueCapacity()
	}
	
	return stats
}
