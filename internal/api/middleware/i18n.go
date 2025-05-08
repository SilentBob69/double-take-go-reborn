package middleware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// I18nConfig definiert die Konfiguration für die i18n-Middleware
type I18nConfig struct {
	DefaultLanguage string
	LocalesDir      string
}

// Übersetzer hält die Übersetzungsfunktionalität
type Translator struct {
	bundle       *i18n.Bundle
	localizer    map[string]*i18n.Localizer
	translations map[string]map[string]interface{}
}

// NewTranslator erstellt einen neuen Übersetzer
func NewTranslator(config I18nConfig) (*Translator, error) {
	// Standardsprache festlegen, falls nicht angegeben
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = "de"
	}

	// Lokalisierungsverzeichnis festlegen, falls nicht angegeben
	if config.LocalesDir == "" {
		config.LocalesDir = "./web/locales"
	}

	// Bundle für die Übersetzung erstellen
	bundle := i18n.NewBundle(language.MustParse(config.DefaultLanguage))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Übersetzer initialisieren
	t := &Translator{
		bundle:       bundle,
		localizer:    make(map[string]*i18n.Localizer),
		translations: make(map[string]map[string]interface{}),
	}

	// Alle Übersetzungsdateien laden
	localeFiles, err := os.ReadDir(config.LocalesDir)
	if err != nil {
		return nil, err
	}

	for _, file := range localeFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			// Sprachcode aus dem Dateinamen extrahieren (z.B. "de.json" -> "de")
			langCode := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			
			// Übersetzungsdatei laden
			filePath := filepath.Join(config.LocalesDir, file.Name())
			_, err := bundle.LoadMessageFile(filePath)
			if err != nil {
				return nil, err
			}

			// Localizer für diese Sprache erstellen
			t.localizer[langCode] = i18n.NewLocalizer(bundle, langCode)
			
			// Vollständige Übersetzungsdatei auch als Map laden für direkten Zugriff
			var translations map[string]interface{}
			jsonData, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			
			if err := json.Unmarshal(jsonData, &translations); err != nil {
				return nil, err
			}
			
			flatTranslations := flattenMap(translations, "")
			t.translations[langCode] = flatTranslations
		}
	}

	return t, nil
}

// I18n erstellt eine Middleware für die Internationalisierung
func I18n(config I18nConfig) gin.HandlerFunc {
	translator, err := NewTranslator(config)
	if err != nil {
		// Im Fehlerfall eine Middleware zurückgeben, die den Fehler protokolliert
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// Sprache aus der Session oder dem Query-Parameter abrufen
		session := sessions.Default(c)
		lang := c.Query("lang")

		// Wenn ein Sprachparameter in der Anfrage vorliegt, diesen in der Session speichern
		if lang != "" && (lang == "de" || lang == "en") {
			session.Set("language", lang)
			session.Save()
		} else {
			// Sprache aus der Session abrufen, falls vorhanden
			sessionLang := session.Get("language")
			if sessionLang != nil {
				lang = sessionLang.(string)
			}
		}

		// Fallback auf die Standardsprache, wenn keine gültige Sprache gefunden wurde
		if lang == "" || (lang != "de" && lang != "en") {
			lang = config.DefaultLanguage
		}

		// Übersetzungsfunktion dem Kontext hinzufügen
		c.Set("language", lang)
		c.Set("translator", translator)
		
		// Template-Funktion für Übersetzungen bereitstellen
		c.Set("t", func(key string, args ...interface{}) string {
			// Direkt aus der Map übersetzen für Effizienz
			if translator.translations[lang] != nil {
				if val, ok := translator.translations[lang][key]; ok {
					return val.(string)
				}
			}
			
			// Fallback auf die Standardsprache
			if translator.translations[config.DefaultLanguage] != nil {
				if val, ok := translator.translations[config.DefaultLanguage][key]; ok {
					return val.(string)
				}
			}
			
			// Wenn kein Wert gefunden wurde, den Schlüssel zurückgeben
			return key
		})

		c.Next()
	}
}

// Flache Map erstellen für einfacheren Zugriff (z.B. "app.title" statt app["title"])
func flattenMap(input map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	
	for k, v := range input {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		
		switch child := v.(type) {
		case map[string]interface{}:
			// Rekursiv verschachtelte Maps abflachen
			flattened := flattenMap(child, key)
			for childKey, childValue := range flattened {
				result[childKey] = childValue
			}
		default:
			// Wert direkt zuordnen
			result[key] = v
		}
	}
	
	return result
}
