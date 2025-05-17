package opencv

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// DebugImage repräsentiert ein Debug-Bild mit Erkennungsdaten
type DebugImage struct {
	ID        string    // Eindeutige ID für das Bild
	Timestamp time.Time // Zeitstempel des Bildes
	ImagePath string    // Originalpfad des Bildes
	ImageData []byte    // Bilddaten mit eingezeichneten Erkennungen
	Persons   int       // Anzahl erkannter Personen
}

// DebugService speichert die letzten OpenCV-Debug-Bilder im Speicher
type DebugService struct {
	images     map[string]*DebugImage // Map von Debug-Bildern, indiziert nach ID
	imagesList []*DebugImage          // Liste für zeitliche Sortierung
	maxImages  int                    // Maximale Anzahl zu speichernder Bilder
	mutex      sync.RWMutex           // Mutex für Thread-Sicherheit
}

// NewDebugService erstellt einen neuen Debug-Service
func NewDebugService(maxImages int) *DebugService {
	if maxImages <= 0 {
		maxImages = 20 // Standardwert falls nicht angegeben
	}
	
	return &DebugService{
		images:     make(map[string]*DebugImage),
		imagesList: make([]*DebugImage, 0, maxImages),
		maxImages:  maxImages,
	}
}

// AddDebugImage fügt ein neues Debug-Bild zum Service hinzu
func (s *DebugService) AddDebugImage(id string, imagePath string, imgData []byte, personsCount int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Debug-Bild erstellen
	debugImg := &DebugImage{
		ID:        id,
		Timestamp: time.Now(),
		ImagePath: imagePath,
		ImageData: imgData,
		Persons:   personsCount,
	}
	
	// Prüfen, ob wir dieses Bild bereits haben
	_, exists := s.images[id]
	if exists {
		// Bild aktualisieren
		s.images[id] = debugImg
		
		// Liste aktualisieren (position finden und durch neues ersetzen)
		for i, img := range s.imagesList {
			if img.ID == id {
				s.imagesList[i] = debugImg
				break
			}
		}
	} else {
		// Neues Bild hinzufügen
		s.images[id] = debugImg
		s.imagesList = append(s.imagesList, debugImg)
		
		// Liste auf maximale Größe begrenzen
		if len(s.imagesList) > s.maxImages {
			// Ältestes Bild entfernen (aus Liste und Map)
			oldest := s.imagesList[0]
			delete(s.images, oldest.ID)
			s.imagesList = s.imagesList[1:]
		}
	}
	
	log.Debugf("Debug-Bild hinzugefügt/aktualisiert: %s mit %d Personen", id, personsCount)
}

// GetLatestImages gibt die neuesten Debug-Bilder zurück
func (s *DebugService) GetLatestImages(count int) []*DebugImage {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	if count <= 0 || count > len(s.imagesList) {
		count = len(s.imagesList)
	}
	
	// Neueste Bilder zurückgeben (die letzten 'count' Elemente)
	result := make([]*DebugImage, count)
	start := len(s.imagesList) - count
	for i := 0; i < count; i++ {
		result[i] = s.imagesList[start+i]
	}
	
	return result
}

// GetImage gibt ein bestimmtes Bild anhand seiner ID zurück
func (s *DebugService) GetImage(id string) *DebugImage {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.images[id]
}

// RegisterRoutes registriert die API-Routen für den Debug-Service
func (s *DebugService) RegisterRoutes(router *gin.Engine) {
	// API-Endpunkte - diese funktionieren bereits
	router.GET("/api/debug/opencv", s.handleGetLatestImages)
	router.GET("/api/debug/opencv/:id", s.handleGetImage)
	
	// Debug-Webseite - wir fügen den Pfad direkt zur Root-Route hinzu
	debug := router.Group("/")
	debug.GET("debug/opencv", s.handleDebugPage)
	
	// Log-Eintrag für Debug-Routing
	log.Infof("OpenCV Debug-Routes registriert: /api/debug/opencv, /api/debug/opencv/:id, /debug/opencv")
}

// handleGetLatestImages gibt die neuesten Debug-Bilder als JSON zurück
func (s *DebugService) handleGetLatestImages(c *gin.Context) {
	countStr := c.DefaultQuery("count", "10")
	count := 10 // Standardwert
	
	// Anzahl parsen, wenn angegeben
	if countStr != "" {
		var err error
		count, err = fmt.Sscanf(countStr, "%d", &count)
		if err != nil {
			count = 10
		}
	}
	
	// Neueste Bilder abrufen
	images := s.GetLatestImages(count)
	
	// Nur die Metadaten zurückgeben, nicht die Bilddaten
	type imageMetadata struct {
		ID        string    `json:"id"`
		Timestamp time.Time `json:"timestamp"`
		ImagePath string    `json:"imagePath"`
		Persons   int       `json:"persons"`
		URL       string    `json:"url"`
	}
	
	metadata := make([]imageMetadata, len(images))
	for i, img := range images {
		// URL-sichere ID für den API-Endpunkt erstellen
		safeID := strings.Replace(img.ID, ".", "-", -1)
		
		metadata[i] = imageMetadata{
			ID:        img.ID,
			Timestamp: img.Timestamp,
			ImagePath: img.ImagePath,
			Persons:   img.Persons,
			URL:       fmt.Sprintf("/api/debug/opencv/%s", safeID),
		}
	}
	
	c.JSON(200, gin.H{
		"count":  len(metadata),
		"images": metadata,
	})
}

// handleGetImage gibt ein bestimmtes Bild zurück
func (s *DebugService) handleGetImage(c *gin.Context) {
	// Holen der URL-sicheren ID aus dem Pfad
	urlSafeID := c.Param("id")
	if urlSafeID == "" {
		c.JSON(400, gin.H{"error": "Keine Bild-ID angegeben"})
		return
	}
	
	// Konvertiere URL-sichere ID zurück zur Original-ID (ersetze Bindestriche durch Punkte)
	originalID := strings.Replace(urlSafeID, "-", ".", -1)
	
	// Bild direkt mit Original-ID suchen
	s.mutex.RLock()
	image := s.images[originalID]
	s.mutex.RUnlock()
	
	// Wenn nicht gefunden, versuche alternative IDs
	if image == nil {
		s.mutex.RLock()
		
		// Alle verfügbaren IDs für Debugging sammeln
		var availableIds []string
		for key := range s.images {
			availableIds = append(availableIds, key)
		}
		
		// Log für Debugging-Zwecke
		log.Debugf("Bild nicht direkt gefunden. Suche: %s (original: %s), verfügbar: %v", 
			urlSafeID, originalID, availableIds)
		
		// Versuche eine übereinstimmende ID zu finden
		for storedID, img := range s.images {
			// Probiere verschiedene Vergleichsmethoden
			if strings.Contains(storedID, originalID) || 
			   strings.Contains(originalID, storedID) || 
			   strings.Replace(storedID, ".", "-", -1) == urlSafeID {
				image = img
				log.Infof("Bild gefunden mit alternativer ID-Übereinstimmung. Angefragt: %s, Gefunden: %s", 
					urlSafeID, storedID)
				break
			}
		}
		
		s.mutex.RUnlock()
	}
	
	// Wenn immer noch nicht gefunden, sende Fehler mit hilfreichen Informationen
	if image == nil {
		c.JSON(404, gin.H{
			"error": "Bild nicht gefunden", 
			"requested_id": urlSafeID,
			"converted_id": originalID,
		})
		return
	}
	
	// Bild als JPEG zurückgeben mit Cache-Kontrolle
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Data(200, "image/jpeg", image.ImageData)
}

// handleDebugPage zeigt die Debug-Seite an
func (s *DebugService) handleDebugPage(c *gin.Context) {
	// HTML-Seite für Debug-Stream zurückgeben mit einem Raw-String-Literal (kein Template-String)
	html := `<!DOCTYPE html>
<html>
<head>
    <title>OpenCV Debug Stream</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background-color: #f0f0f0; }
        h1 { color: #333; }
        .container { max-width: 1200px; margin: 0 auto; }
        .image-container { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 20px; }
        .image-card { background: white; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); overflow: hidden; width: 300px; }
        .image-card img { width: 100%; height: auto; max-height: 300px; object-fit: contain; }
        .image-info { padding: 10px; border-top: 1px solid #eee; }
        .controls { margin: 20px 0; }
        button { padding: 8px 15px; background: #2c3e50; color: white; border: none; border-radius: 4px; cursor: pointer; }
        button:hover { background: #34495e; }
        .refresh-timer { display: inline-block; margin-left: 15px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>OpenCV Debug-Stream</h1>
        <p>Diese Seite zeigt die OpenCV-Personenerkennung in Echtzeit an.</p>
        
        <div class="controls">
            <button id="refresh-button">Jetzt aktualisieren</button>
            <span class="refresh-timer">Automatische Aktualisierung in <span id="countdown">10</span>s</span>
            <label style="margin-left: 20px">
                <input type="checkbox" id="auto-refresh" checked> Auto-Aktualisierung
            </label>
        </div>
        
        <div class="image-container" id="image-container">
            <p>Lade Bilder...</p>
        </div>
    </div>

    <script>
        // Variablen
        let countdown = 10;
        let timer = null;
        let autoRefresh = true;
        
        // DOM-Elemente
        const imageContainer = document.getElementById('image-container');
        const refreshButton = document.getElementById('refresh-button');
        const countdownElement = document.getElementById('countdown');
        const autoRefreshCheckbox = document.getElementById('auto-refresh');
        
        // Event-Listener
        refreshButton.addEventListener('click', fetchImages);
        autoRefreshCheckbox.addEventListener('change', function(e) {
            autoRefresh = e.target.checked;
            if (autoRefresh) {
                startCountdown();
            } else {
                clearTimeout(timer);
                countdownElement.textContent = '—';
            }
        });
        
        // Bilder vom Server laden
        function fetchImages() {
            fetch('/api/debug/opencv?count=20')
                .then(function(response) { return response.json(); })
                .then(function(data) {
                    if (data.count === 0) {
                        imageContainer.innerHTML = '<p>Keine Bilder vorhanden. Warten auf neue Erkennungen...</p>';
                        return;
                    }
                    
                    imageContainer.innerHTML = '';
                    data.images.sort(function(a, b) { 
                        return new Date(b.timestamp) - new Date(a.timestamp);
                    });
                    
                    data.images.forEach(function(image) {
                        const card = document.createElement('div');
                        card.className = 'image-card';
                        
                        const img = document.createElement('img');
                        img.src = image.url + '?t=' + new Date().getTime(); // Cache-Busting
                        img.alt = 'Debug-Bild';
                        img.loading = 'lazy';
                        
                        const info = document.createElement('div');
                        info.className = 'image-info';
                        const time = new Date(image.timestamp).toLocaleTimeString();
                        info.innerHTML = 
                            "<div><b>Personen erkannt:</b> " + image.persons + "</div>" +
                            "<div><b>Zeit:</b> " + time + "</div>" +
                            "<div><b>Bild:</b> " + image.imagePath.split('/').pop() + "</div>";
                        
                        card.appendChild(img);
                        card.appendChild(info);
                        imageContainer.appendChild(card);
                    });
                })
                .catch(function(error) {
                    console.error('Fehler beim Laden der Bilder:', error);
                    imageContainer.innerHTML = '<p>Fehler beim Laden der Bilder. Bitte versuche es erneut.</p>';
                })
                .finally(function() {
                    if (autoRefresh) {
                        countdown = 10;
                        startCountdown();
                    }
                });
        }
        
        // Countdown für die nächste Auto-Aktualisierung
        function startCountdown() {
            clearTimeout(timer);
            countdownElement.textContent = countdown;
            
            if (countdown <= 0) {
                fetchImages();
                return;
            }
            
            timer = setTimeout(function() {
                countdown--;
                startCountdown();
            }, 1000);
        }
        
        // Initialisierung
        fetchImages();
    </script>
</body>
</html>`
	
	c.Header("Content-Type", "text/html")
	c.String(200, html)
}
