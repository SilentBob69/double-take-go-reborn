package processor

import (
	"double-take-go-reborn/internal/util/timezone"
	"context"
	"runtime"
	"sync"
	"time"

	"double-take-go-reborn/internal/core/models"

	log "github.com/sirupsen/logrus"
)

// WorkerPool verwaltet einen Pool von Worker-Goroutinen für die Bildverarbeitung
type WorkerPool struct {
	processor      *ImageProcessor
	jobs           chan *ProcessJob
	results        chan *ProcessResult
	workerCount    int
	activeJobs     int
	activeJobsMutex sync.Mutex
	shutdown       chan struct{}
}

// ProcessJob repräsentiert einen Bildverarbeitungsjob
type ProcessJob struct {
	ctx       context.Context
	imagePath string
	source    string
	options   ProcessingOptions
	resultCh  chan *ProcessResult // Individueller Ergebniskanal pro Job
}

// ProcessResult enthält das Ergebnis der Bildverarbeitung
type ProcessResult struct {
	Image *models.Image
	Err   error
}

// NewWorkerPool erstellt einen neuen Worker-Pool für die Bildverarbeitung
func NewWorkerPool(processor *ImageProcessor) *WorkerPool {
	// Container-bewusste Konfiguration: Verwende 75% der verfügbaren CPUs, mindestens 2
	availableCPUs := runtime.NumCPU()
	workerCount := max(2, (availableCPUs * 3) / 4)
	
	log.Infof("Initializing image processing worker pool with %d workers", workerCount)
	
	pool := &WorkerPool{
		processor:   processor,
		jobs:        make(chan *ProcessJob, workerCount*2), // Puffer für Jobs
		results:     make(chan *ProcessResult, workerCount*2), // Puffer für Ergebnisse
		workerCount: workerCount,
		shutdown:    make(chan struct{}),
	}
	
	// Workers starten
	pool.startWorkers()
	
	return pool
}

// startWorkers startet die Worker-Goroutinen
func (p *WorkerPool) startWorkers() {
	for i := 0; i < p.workerCount; i++ {
		go func(workerID int) {
			log.Debugf("Worker %d started", workerID)
			
			for {
				select {
				case job, ok := <-p.jobs:
					if !ok {
						log.Debugf("Worker %d shutting down (job channel closed)", workerID)
						return
					}
					
					// Job-Zähler erhöhen
					p.activeJobsMutex.Lock()
					p.activeJobs++
					jobCount := p.activeJobs
					p.activeJobsMutex.Unlock()
					
					log.Debugf("Worker %d processing image from %s (active jobs: %d)", 
						workerID, job.source, jobCount)
					
					startTime := timezone.Now()
					
					// Bild verarbeiten
					image, err := p.processor.processImageInternal(
						job.ctx, job.imagePath, job.source, job.options)
					
					// Job-Zähler reduzieren
					p.activeJobsMutex.Lock()
					p.activeJobs--
					p.activeJobsMutex.Unlock()
					
					// Ergebnis liefern
					result := &ProcessResult{
						Image: image,
						Err:   err,
					}
					
					// Direkt an den anfragenden Goroutine senden
					select {
					case job.resultCh <- result:
						// Ergebnis erfolgreich gesendet
					default:
						log.Warnf("Worker %d: Could not send result, channel might be closed", workerID)
					}
					
					elapsed := time.Since(startTime)
					log.Infof("Worker %d completed image processing in %v", workerID, elapsed)
					
				case <-p.shutdown:
					log.Debugf("Worker %d received shutdown signal", workerID)
					return
				}
			}
		}(i)
	}
}

// ProcessImage verarbeitet ein Bild asynchron über den Worker-Pool
func (p *WorkerPool) ProcessImage(ctx context.Context, imagePath, source string, 
	options ProcessingOptions) (*models.Image, error) {
	
	// Ergebniskanal für diesen spezifischen Job
	resultCh := make(chan *ProcessResult, 1)
	
	// Job erstellen
	job := &ProcessJob{
		ctx:       ctx,
		imagePath: imagePath,
		source:    source,
		options:   options,
		resultCh:  resultCh,
	}
	
	// Job an den Pool senden
	select {
	case p.jobs <- job:
		// Job angenommen
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	
	// Auf Ergebnis warten
	select {
	case result := <-resultCh:
		return result.Image, result.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ActiveJobCount gibt die Anzahl der aktuell aktiven Jobs zurück
func (p *WorkerPool) ActiveJobCount() int {
	p.activeJobsMutex.Lock()
	defer p.activeJobsMutex.Unlock()
	return p.activeJobs
}

// GetWorkerCount gibt die Anzahl der Worker im Pool zurück
func (p *WorkerPool) GetWorkerCount() int {
	return p.workerCount
}

// GetQueueCapacity gibt die Kapazität der Job-Queue zurück
func (p *WorkerPool) GetQueueCapacity() int {
	return cap(p.jobs)
}

// Shutdown fährt den Worker-Pool herunter
func (p *WorkerPool) Shutdown() {
	close(p.shutdown)
	close(p.jobs)
}

// Hilfsfunktion max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
