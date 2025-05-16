package handlers

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/gin-gonic/gin"
)

// RestartContainer startet den Container neu
func (h *APIHandler) RestartContainer(c *gin.Context) {
	// Antwort an den Client senden, bevor der Neustart beginnt
	c.JSON(200, gin.H{
		"success": true,
		"message": "Container wird neu gestartet...",
	})
	// Flush der Antwort sicherstellen
	if flusher, ok := c.Writer.(interface{ Flush() }); ok {
		flusher.Flush()
	}

	// Neustart im Hintergrund ausführen, damit die Antwort an den Client gesendet werden kann
	go func() {
		log.Println("Container-Neustart wird eingeleitet...")
		
		// Prozess-ID des aktuellen Prozesses
		pid := os.Getpid()
		log.Printf("Aktueller Prozess-ID: %d", pid)

		// Führe einen Neustart durch, indem ein SIGTERM-Signal an den Prozess gesendet wird
		// Docker wird den Container dann automatisch neu starten, wenn er entsprechend konfiguriert ist
		proc, err := os.FindProcess(pid)
		if err != nil {
			log.Printf("Fehler beim Finden des Prozesses: %v", err)
			
			// Alternativer Ansatz: Versuche einen Neustart über die Docker-API
			cmd := exec.Command("kill", "-15", "1") // Signal an PID 1 im Container senden
			err = cmd.Run()
			if err != nil {
				log.Printf("Fehler beim Ausführen des Neustartbefehls: %v", err)
				return
			}
		} else {
			// Sende SIGTERM an den Prozess
			err = proc.Signal(syscall.SIGTERM)
			if err != nil {
				log.Printf("Fehler beim Senden des Signals: %v", err)
				return
			}
			log.Println("SIGTERM-Signal erfolgreich gesendet")
		}
	}()
}
