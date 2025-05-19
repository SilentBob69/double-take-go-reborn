package opencv

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"double-take-go-reborn/config"
	gocv "gocv.io/x/gocv"
	log "github.com/sirupsen/logrus"
)

// Detektionstypen für Personenerkennung
const (
	HOGDetector = "hog"      // Histogram of Oriented Gradients Detektor (CPU)
	DNNDetector = "dnn"      // DNN basierter Personen-Detektor (genauer, kann GPU nutzen)
)

// DNN-Modelltypen
const (
	SSDMobileNet  = "ssd_mobilenet" // MobileNet für schnelle Erkennung
	YOLOv3       = "yolov3"         // YOLOv3 (genauer, aber größer)
	YOLOv4       = "yolov4"         // YOLOv4 (genauer, aber größer)
)

// DNN-Backend-Typen für die Konfiguration
const (
	BackendDefault = "default"
	BackendCUDA    = "cuda"
	BackendOpenCL  = "opencl"
	TargetDefault  = "default"
	TargetCPU      = "cpu"
	TargetCUDA     = "cuda"
	TargetOpenCL   = "opencl"
)

// Backend-Konstanten für OpenCV DNN
const (
	// OpenCV Backend-Typen als int für die gocv-Bibliothek
	NetBackendDefault = 0   // Entspricht gocv.NetBackendDefault
	NetBackendHalide  = 1   // Entspricht gocv.NetBackendHalide
	NetBackendInferenceEngine = 2 // Entspricht gocv.NetBackendInferenceEngine
	NetBackendOpenVINO = 5  // Entspricht gocv.NetBackendOpenVINO
	NetBackendOpenCV = 3    // Entspricht gocv.NetBackendOpenCV
	NetBackendVKCOM = 4     // Entspricht gocv.NetBackendVKCOM
	NetBackendCUDA = 6      // Entspricht gocv.NetBackendCUDA
	NetBackendOpenCL = 7    // Nicht direkt in gocv definiert
	
	// Target-Typen
	NetTargetCPU = 0      // Entspricht gocv.NetTargetCPU
	NetTargetFP32 = 1     // Entspricht gocv.NetTargetFP32
	NetTargetFP16 = 2     // Entspricht gocv.NetTargetFP16
	NetTargetVPU = 3      // Entspricht gocv.NetTargetVPU
	NetTargetVULKAN = 4   // Entspricht gocv.NetTargetVULKAN
	NetTargetFPGA = 5     // Entspricht gocv.NetTargetFPGA
	NetTargetCUDA = 6     // Entspricht gocv.NetTargetCUDA
	NetTargetCUDAFP16 = 7 // Entspricht gocv.NetTargetCUDAFP16
	NetTargetOpenCL = 8   // Nicht direkt in gocv definiert
)   // OpenCL (für AMD-GPUs)

// Weitere Konstanten
const (
	// Standardwerte für DNN-Modell
	DefaultDNNWidth = 300
	DefaultDNNHeight = 300
)

// PersonDetector implementiert die Personenerkennung mit OpenCV
type PersonDetector struct {
	cfg                *config.OpenCVConfig  // Konfiguration
	detectorType      string                // Typ des Detektors (HOG oder DNN)
	dnnModelType      string                // Typ des DNN-Modells bei DNN-Detektor
	hogDescriptor     gocv.HOGDescriptor     // HOG-Detektor für Standard-CPU-Erkennung
	dnnNet            gocv.Net               // DNN-Netzwerk für präzisere Erkennung
	backend           int                    // Backend für DNN (z.B. CUDA, OpenCL)
	target            int                    // Target für DNN (z.B. CUDA, CPU)
	initialized       bool                   // Flag ob Detektor initialisiert ist
	classNames        []string               // Klassennamen für DNN-Detektoren
	personClassId     int                    // ID der Personenklasse im Modell
	confidenceThreshold float64              // Schwellenwert für die Erkennungskonfidenz
	useGPU            bool                   // Flag für GPU-Nutzung
	dnnInputWidth     int                    // Breite des Eingabebildes für DNN
	dnnInputHeight    int                    // Höhe des Eingabebildes für DNN
	debugService      *DebugService          // Verweis auf den Debug-Service für Visualisierung
}

// DetectedPerson repräsentiert eine erkannte Person mit Position und Konfidenz
type DetectedPerson struct {
	Rectangle  image.Rectangle // Position und Größe der Person
	Confidence float64         // Konfidenzwert der Erkennung
}

// NewPersonDetector erstellt einen neuen Personendetektor
func NewPersonDetector(cfg *config.OpenCVConfig) (*PersonDetector, error) {
	// Standardwerte festlegen
	detectorType := HOGDetector
	if cfg.PersonDetection.Method != "" {
		detectorType = cfg.PersonDetection.Method
	}

	// Bei GPU-Verwendung DNN empfehlen
	if cfg.UseGPU && detectorType == HOGDetector {
		log.Warn("GPU-Beschleunigung ist konfiguriert, aber HOG-Detektor gewählt. " +
			"Für GPU-Beschleunigung wird DNN-Detektor empfohlen.")
	}

	modelType := SSDMobileNet
	if cfg.PersonDetection.Model != "" {
		modelType = cfg.PersonDetection.Model
	}

	// Backend und Target basierend auf Konfiguration und Plattform wählen
	backend, target := getGPUBackend(*cfg)

	detector := &PersonDetector{
		cfg:                cfg,
		detectorType:       detectorType,
		dnnModelType:       modelType,
		backend:            backend,
		target:             target,
		initialized:        false,
		personClassId:      0, // Dies ist die Standard-ID für "person" in COCO-Datensatz
		confidenceThreshold: cfg.PersonDetection.ConfidenceThreshold,
		useGPU:             cfg.UseGPU,
		dnnInputWidth:      DefaultDNNWidth,
		dnnInputHeight:     DefaultDNNHeight,
	}

	if detector.confidenceThreshold <= 0 {
		detector.confidenceThreshold = 0.5 // Standardwert, falls nicht konfiguriert
	}

	// DNN-Modelle werden durch die Service-Klasse initialisiert
	// Die Initialize-Methode wird erst später aufgerufen

	return detector, nil
}

// getGPUBackend gibt das zu verwendende Backend und Target basierend auf der Konfiguration zurück
func getGPUBackend(cfg config.OpenCVConfig) (int, int) {
	backend := NetBackendDefault
	target := NetTargetCPU
	
	configBackend := cfg.PersonDetection.Backend
	configTarget := cfg.PersonDetection.Target
	
	if configBackend == "" || configBackend == BackendDefault {
		// Plattform-spezifische Erkennung
		if cfg.UseGPU {
			// NVIDIA GPU-Erkennung
			if haveNvidiaGPU() {
				backend = NetBackendCUDA
				target = NetTargetCUDA
				log.Info("NVIDIA GPU erkannt, verwende CUDA-Backend")
				return backend, target
			}
			
			// AMD GPU-Erkennung
			if haveAMDGPU() {
				backend = NetBackendOpenCL
				target = NetTargetOpenCL
				log.Info("AMD GPU erkannt, verwende OpenCL-Backend")
				return backend, target
			}
			
			// Apple Silicon-Optimierung
			if runtime.GOOS == "darwin" && (runtime.GOARCH == "arm64" || strings.Contains(runtime.GOARCH, "arm")) {
				// Für Apple Silicon ist derzeit die optimierte CPU-Variante am besten
				log.Info("Apple Silicon erkannt, verwende optimierte CPU-Version")
				// Standardwerte beibehalten, da Metal noch nicht direkt unterstützt wird
				return backend, target
			}
			
			log.Warn("GPU-Nutzung aktiviert, aber keine unterstützte GPU erkannt. Verwende CPU.")
		}
	} else {
		// Explizite Backend-Konfiguration
		switch configBackend {
		case BackendCUDA:
			backend = NetBackendCUDA
		case BackendOpenCL:
			backend = NetBackendOpenCL
		default:
			log.Warnf("Unbekanntes Backend '%s' konfiguriert, verwende Standard", configBackend)
		}
		
		// Explizite Target-Konfiguration
		switch configTarget {
		case TargetCUDA:
			target = NetTargetCUDA
		case TargetOpenCL:
			target = NetTargetOpenCL
		case TargetCPU:
			target = NetTargetCPU
		default:
			log.Warnf("Unbekanntes Target '%s' konfiguriert, verwende CPU", configTarget)
			target = NetTargetCPU
		}
	}

	return backend, target
}

// haveNvidiaGPU prüft, ob eine NVIDIA-GPU verfügbar ist
func haveNvidiaGPU() bool {
	// Prüfe zuerst nach NVIDIA-Docker-Umgebungsvariablen
	if os.Getenv("NVIDIA_VISIBLE_DEVICES") != "" || os.Getenv("NVIDIA_DRIVER_CAPABILITIES") != "" {
		log.Info("NVIDIA-Docker-Umgebung erkannt über Umgebungsvariablen")
		return true
	}

	// Prüfe CUDA-Bibliotheken im Container
	cudaLibPaths := []string{
		"/usr/local/cuda/lib64/libcudart.so",
		"/usr/lib/x86_64-linux-gnu/libcuda.so",
		"/usr/lib/libcuda.so",
	}
	
	for _, path := range cudaLibPaths {
		if _, err := os.Stat(path); err == nil {
			log.Infof("CUDA-Bibliothek gefunden: %s", path)
			return true
		}
	}

	// Standardmethode: Prüfen, ob nvidia-smi existiert und ausführbar ist
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		nvidiaSmiPaths := []string{
			"/usr/bin/nvidia-smi",
			"/usr/local/bin/nvidia-smi",
			"/bin/nvidia-smi",
		}
		
		for _, path := range nvidiaSmiPaths {
			if _, err := os.Stat(path); err == nil {
				log.Infof("nvidia-smi gefunden: %s", path)
				return true
			}
		}
	} else if runtime.GOOS == "windows" {
		// Windows-Pfade zu nvidia-smi prüfen
		windowsPaths := []string{
			"C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
			"C:\\Windows\\System32\\nvidia-smi.exe",
		}
		for _, path := range windowsPaths {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// haveAMDGPU prüft, ob eine AMD-GPU verfügbar ist
func haveAMDGPU() bool {
	// Einfache Heuristik für Linux/macOS: Prüfen, ob AMD-spezifische Dateien existieren
	if runtime.GOOS == "linux" {
		paths := []string{
			"/dev/kfd",
			"/dev/dri/renderD128",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	} else if runtime.GOOS == "darwin" {
		// macOS hat keine einfache Möglichkeit, AMD-GPUs zu erkennen
		// In Zukunft könnte hier eine aufwändigere Erkennung implementiert werden
	}
	return false
}

// Initialize initialisiert die Detektoren basierend auf der Konfiguration
func (pd *PersonDetector) Initialize(ctx context.Context) error {
	if pd.initialized {
		return nil
	}

	log.Infof("Initialisiere OpenCV Personenerkennung (Methode: %s, GPU: %v)", 
		pd.detectorType, pd.cfg.UseGPU)

	// Je nach konfiguriertem Detektortyp initialisieren
	if pd.detectorType == HOGDetector {
		// HOG-Detektor ist einfacher zu initialisieren
		hogDesc := gocv.NewHOGDescriptor()
		hogDesc.SetSVMDetector(gocv.HOGDefaultPeopleDetector())
		pd.hogDescriptor = hogDesc
		log.Info("HOG-Personen-Detektor erfolgreich initialisiert")
	} else if pd.detectorType == DNNDetector {
		// DNN-basierte Personenerkennung initialisieren
		var modelPath, configPath string
		
		// Je nach Modelltyp die richtigen Dateien laden
		switch pd.dnnModelType {
		case SSDMobileNet:
			// Prüfen ob die Modelldateien existieren, wenn nicht, warnen und auf HOG zurückfallen
			modelPath = pd.cfg.PersonDetection.ModelPath
			if modelPath == "" {
				// Standard-Modellpfad ist in models/opencv
				modelPath = filepath.Join("models", "opencv", "ssd_mobilenet_v3_large_coco_2020_01_14.pb")
			}
			
			configPath = pd.cfg.PersonDetection.ConfigPath
			if configPath == "" {
				configPath = filepath.Join("models", "opencv", "ssd_mobilenet_v3_large_coco_2020_01_14.pbtxt")
			}
			
			// Klassenliste für COCO-Datensatz initialisieren
			pd.classNames = []string{
				"background", "person", "bicycle", "car", "motorcycle", "airplane", "bus", "train",
				"truck", "boat", "traffic light", "fire hydrant", "stop sign", "parking meter", "bench",
				"bird", "cat", "dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe",
				"backpack", "umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard",
				"sports ball", "kite", "baseball bat", "baseball glove", "skateboard", "surfboard", 
				"tennis racket", "bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl",
				"banana", "apple", "sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza",
				"donut", "cake", "chair", "couch", "potted plant", "bed", "dining table", "toilet",
				"tv", "laptop", "mouse", "remote", "keyboard", "cell phone", "microwave", "oven", 
				"toaster", "sink", "refrigerator", "book", "clock", "vase", "scissors", "teddy bear",
				"hair drier", "toothbrush",
			}
			
			// Person ist Klasse 1 in COCO
			pd.personClassId = 1
			
		case YOLOv4:
			// YOLO-Modelldateien
			modelPath = pd.cfg.PersonDetection.ModelPath
			if modelPath == "" {
				modelPath = filepath.Join("models", "opencv", "yolov4.weights")
			}
			
			configPath = pd.cfg.PersonDetection.ConfigPath
			if configPath == "" {
				configPath = filepath.Join("models", "opencv", "yolov4.cfg")
			}
			
			// YOLO verwendet COCO-Klassen
			pd.classNames = []string{
				"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train",
				"truck", "boat", "traffic light", "fire hydrant", "stop sign", "parking meter", "bench",
				// usw. - vollständige Liste für YOLO würde hier fortgesetzt
			}
			
			// Person ist Klasse 0 in YOLO
			pd.personClassId = 0
		}
		
		// Prüfen, ob die Dateien existieren
		if !fileExists(modelPath) || !fileExists(configPath) {
			log.Warnf("DNN-Modelldateien nicht gefunden: %s oder %s", modelPath, configPath)
			log.Warn("Falle zurück auf HOG-Detektor")
			pd.detectorType = HOGDetector
			hogDesc := gocv.NewHOGDescriptor()
			hogDesc.SetSVMDetector(gocv.HOGDefaultPeopleDetector())
			pd.hogDescriptor = hogDesc
		} else {
			// DNN-Modell laden
			net := gocv.ReadNet(modelPath, configPath)
			if net.Empty() {
				return fmt.Errorf("konnte DNN-Modell nicht laden: %s", modelPath)
			}
			
			// Setze Backend-Optionen wenn GPU aktiviert ist
			if pd.cfg.UseGPU {
				backend, target := getGPUBackend(*pd.cfg)
				pd.dnnNet.SetPreferableBackend(gocv.NetBackendType(backend))
				pd.dnnNet.SetPreferableTarget(gocv.NetTargetType(target))
				log.Debugf("DNN nutzt GPU-Backend: %v, Target: %v", backend, target)
			}
			
			// Loggen, welches Backend tatsächlich verwendet wird
			log.Infof("DNN-Modell geladen mit Backend %d und Target %d", pd.backend, pd.target)
			
			pd.dnnNet = net
		}
	}
	
	pd.initialized = true
	return nil
}

// DetectPersons erkennt Personen in einem Bild
func (pd *PersonDetector) DetectPersons(ctx context.Context, imgPath string) ([]DetectedPerson, error) {
	if !pd.initialized {
		return nil, fmt.Errorf("PersonDetector ist nicht initialisiert")
	}

	// Bild laden
	img := gocv.IMRead(imgPath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("konnte Bild nicht laden: %s", imgPath)
	}
	defer img.Close()

	var persons []DetectedPerson

	// Bild für Performance skalieren wenn nötig
	var processImg gocv.Mat
	defer func() {
		if !processImg.Empty() {
			processImg.Close()
		}
	}()

	imgWidth := img.Cols()
	imgHeight := img.Rows()
	const maxDimension = 800 // Maximum für schnellere Verarbeitung

	if imgWidth > maxDimension || imgHeight > maxDimension {
		// Berechne den Skalierungsfaktor
		scale := float64(maxDimension) / float64(max(imgWidth, imgHeight))
		newWidth := int(float64(imgWidth) * scale)
		newHeight := int(float64(imgHeight) * scale)
		
		processImg = gocv.NewMat()
		gocv.Resize(img, &processImg, image.Point{X: newWidth, Y: newHeight}, 0, 0, gocv.InterpolationLinear)
	} else {
		processImg = img.Clone()
	}

	// Je nach konfiguriertem Detektor
	if pd.detectorType == HOGDetector {
		// HOG-basierte Personenerkennung mit konfigurierten Parametern
		// Personen mit HOG-Descriptor erkennen
		// Angepasste Parameter - ohne zusätzliche Konfigurationsparameter
		rects := pd.hogDescriptor.DetectMultiScale(processImg)
		
		// HOG liefert keine direkten Konfidenzwerte, daher setzen wir 0.8 als Standard
		weights := make([]float64, len(rects))
		for i := range weights {
			weights[i] = 0.8 // Standard-Konfidenz für HOG
		}

		// Skalierung zurück, wenn Bild verkleinert wurde
		scaleX := float64(imgWidth) / float64(processImg.Cols())
		scaleY := float64(imgHeight) / float64(processImg.Rows())

		// Alle gefundenen Personen in die Ergebnisliste einfügen
		for i, r := range rects {
			// Bei verkleinertem Bild die Koordinaten zurückskalieren
			if scaleX != 1.0 || scaleY != 1.0 {
				r = image.Rect(
					int(float64(r.Min.X)*scaleX),
					int(float64(r.Min.Y)*scaleY),
					int(float64(r.Max.X)*scaleX),
					int(float64(r.Max.Y)*scaleY),
				)
			}

			// Konfidenzwert aus HOG-Detektor verwenden
			confidence := pd.confidenceThreshold // Standardwert aus der Konfiguration verwenden
			if i < len(weights) {
				confidence = weights[i] // Tatsächlicher Konfidenzwert aus der HOG-Erkennung
			}

			// Nur hinzufügen, wenn Konfidenz über dem Schwellenwert
			if confidence >= pd.confidenceThreshold {
				person := DetectedPerson{
					Rectangle:  r,
					Confidence: confidence,
				}
				persons = append(persons, person)
			}
		}
	} else if pd.detectorType == DNNDetector && !pd.dnnNet.Empty() {
		// DNN-basierte Personenerkennung
		// Bild in Blob umwandeln für DNN
		blob := gocv.BlobFromImage(
			processImg,
			1.0,                                       // Scalefactor
			image.Point{pd.dnnInputWidth, pd.dnnInputHeight}, // Größe aus Konfiguration
			gocv.NewScalar(127.5, 127.5, 127.5, 0),    // Mean - normalisieren auf [-1,1]
			true,                                      // SwapRB - BGR zu RGB
			false,                                     // Crop
		)
		defer blob.Close()

		// Forward pass durch das Netzwerk
		pd.dnnNet.SetInput(blob, "")
		prob := pd.dnnNet.Forward("")
		defer prob.Close()

		// Ergebnisse verarbeiten
		rows := prob.Rows()
		// SSD-Format interpretieren: [img_id, class_id, confidence, left, top, right, bottom]
		for i := 0; i < rows; i++ {
			confidence := prob.GetFloatAt(i, 2)
			
			// Nur weitermachen, wenn die Konfidenz über dem Schwellenwert liegt
			if float64(confidence) < pd.confidenceThreshold {
				continue
			}
			
			// Klassen-ID prüfen - wir interessieren uns nur für Personen
			classID := int(prob.GetFloatAt(i, 1))
			if classID != pd.personClassId {
				continue
			}
			
			// Konfidenzwert auslesen und prüfen
			conf := prob.GetFloatAt(i, 2)
			if float64(conf) < pd.confidenceThreshold {
				continue
			}
			
			// Bounding Box extrahieren
			left := int(prob.GetFloatAt(i, 3) * float32(imgWidth))
			top := int(prob.GetFloatAt(i, 4) * float32(imgHeight))
			right := int(prob.GetFloatAt(i, 5) * float32(imgWidth))
			bottom := int(prob.GetFloatAt(i, 6) * float32(imgHeight))
			
			// Rechteck erstellen
			rect := image.Rect(left, top, right, bottom)
			
			person := DetectedPerson{
				Rectangle:  rect,
				Confidence: float64(conf),
			}
			persons = append(persons, person)
		}
	}

	log.Debugf("OpenCV: %d Personen in %s erkannt", len(persons), filepath.Base(imgPath))
	
	// Visualisierung der erkannten Personen für Debug-Stream
	log.Debugf("Beginne Visualisierung für %d erkannte Personen", len(persons))
	
	// Zugriff auf Debug-Service prüfen
	if pd.debugService == nil {
		log.Debug("Kein Debug-Service verfügbar für Visualisierung")
		return persons, nil
	}
	
	if len(persons) > 0 {
		try := func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Panic bei der Visualisierung: %v", r)
				}
			}()
			
			// Originalbild für die Visualisierung verwenden
			log.Debugf("Klone Bild für Visualisierung: %s", imgPath)
			visImg := img.Clone()
			defer visImg.Close()
			
			if visImg.Empty() {
				log.Errorf("Visualisierungsbild ist leer!")
				return
			}
			
			log.Debugf("Beginne Zeichnen von %d Rechtecken", len(persons))
			
			// Rechtecke für alle erkannten Personen einzeichnen
			for i, person := range persons {
				r := person.Rectangle
				
				// Rechteck mit roter Farbe zeichnen (GoCV erwartet Scalar, nicht color.RGBA)
				red := color.RGBA{255, 0, 0, 0} 
				gocv.Rectangle(&visImg, r, red, 2)
				
				// Konfidenzwert als Text anzeigen
				confText := fmt.Sprintf("Person %d: %.2f", i+1, person.Confidence)
				green := color.RGBA{0, 255, 0, 0}
				gocv.PutText(&visImg, confText, image.Point{
					X: r.Min.X,
					Y: r.Min.Y - 5,
				}, gocv.FontHersheyPlain, 1.2, green, 2)
			}
			
			// Bild in JPEG-Format encodieren für Speicherung im Debug-Service
			buf, err := gocv.IMEncode(".jpg", visImg)
			if err != nil {
				log.Errorf("Konnte Bild nicht encodieren: %v", err)
				return
			}
			
			// Native Buffer in []byte umwandeln
			imgBytes := buf.GetBytes()
			
			// Eindeutige ID für das Debug-Bild generieren
			baseName := filepath.Base(imgPath)
			imageID := strings.TrimSuffix(baseName, filepath.Ext(baseName))
			
			// Debug-Bild zum Service hinzufügen
			pd.debugService.AddDebugImage(imageID, imgPath, imgBytes, len(persons))
			log.Infof("Debug-Bild für OpenCV-Erkennung hinzugefügt: %s mit %d Personen", imageID, len(persons))
		}
		
		try()
	}
	
	return persons, nil
}

// Close gibt Ressourcen frei
func (pd *PersonDetector) Close() error {
	if pd.initialized {
		if !pd.dnnNet.Empty() {
			pd.dnnNet.Close()
		}
		// HOGDescriptor benötigt keinen expliziten Close-Aufruf
		pd.initialized = false
	}
	return nil
}

// Hilfsfunktion zur Überprüfung, ob eine Datei existiert
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Hilfsfunktion für max von zwei int-Werten
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
