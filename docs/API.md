# Double-Take Go API-Dokumentation

Diese Dokumentation beschreibt die REST-API des Double-Take Go-Systems, die von externen Anwendungen genutzt werden kann, um mit dem System zu interagieren.

## Basis-URL

Alle API-Endpunkte sind relativ zur Basis-URL des Servers, standardmäßig:

```
http://localhost:3000/api
```

## API-Übersicht

Die API ist in folgende Hauptbereiche unterteilt:

- **Verarbeitungs-Endpunkte**: Für die Verarbeitung neuer Bilder
- **Bilder-Endpunkte**: Zum Verwalten und Abfragen von Bildern
- **Identitäts-Endpunkte**: Zum Verwalten von erkannten Personen/Identitäten
- **System-Endpunkte**: Für Systemfunktionen und -status

## Verarbeitungs-Endpunkte

### Bild verarbeiten

Lädt ein neues Bild hoch und verarbeitet es zur Gesichtserkennung.

- **URL**: `/process/image`
- **Methode**: `POST`
- **Content-Type**: `multipart/form-data`

**Formular-Parameter:**

| Parameter | Typ   | Beschreibung                  |
|-----------|-------|-------------------------------|
| file      | Datei | Das zu verarbeitende Bild     |
| source    | Text  | Quelle des Bildes (optional)  |

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "image_id": 1,
    "faces": [
      {
        "id": 1,
        "box": {
          "x_min": 100,
          "y_min": 200,
          "x_max": 300,
          "y_max": 400
        },
        "matches": [
          {
            "identity_id": 1,
            "name": "Person1",
            "confidence": 0.85
          }
        ]
      }
    ]
  }
  ```

**Fehlerantwort:**

- **Code**: 400 Bad Request
- **Inhalt**:
  ```json
  {
    "error": "Keine Datei hochgeladen"
  }
  ```

### CompreFace-Verarbeitung

Verarbeitet ein Bild direkt mit CompreFace.

- **URL**: `/process/compreface`
- **Methode**: `POST`
- **Content-Type**: `multipart/form-data`

**Formular-Parameter:**

| Parameter | Typ   | Beschreibung              |
|-----------|-------|---------------------------|
| file      | Datei | Das zu verarbeitende Bild |

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**: CompreFace-Antwort im JSON-Format

## Bilder-Endpunkte

### Bilder auflisten

Ruft eine Liste aller Bilder ab, mit optionaler Filterung.

- **URL**: `/images`
- **Methode**: `GET`

**Query-Parameter:**

| Parameter  | Beschreibung                                         |
|------------|----------------------------------------------------- |
| page       | Seitennummer (Standard: 1)                           |
| limit      | Anzahl der Einträge pro Seite (Standard: 20)         |
| source     | Nach Quelle filtern (z.B. "frigate", "upload")       |
| hasfaces   | Nur Bilder mit/ohne Gesichter ("yes"/"no")           |
| hasmatches | Nur Bilder mit/ohne Treffer ("yes"/"no")             |
| daterange  | Zeitraum ("today", "yesterday", "week", "month")     |

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "images": [
      {
        "id": 1,
        "file_path": "uploads/image1.jpg",
        "source": "upload",
        "timestamp": "2025-05-08T16:30:00Z",
        "face_count": 2,
        "has_matches": true
      }
    ],
    "pagination": {
      "current": 1,
      "total": 5,
      "total_items": 100
    }
  }
  ```

### Bild abrufen

Ruft detaillierte Informationen zu einem bestimmten Bild ab.

- **URL**: `/images/:id`
- **Methode**: `GET`
- **URL-Parameter**: `id` - ID des Bildes

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "id": 1,
    "file_path": "uploads/image1.jpg",
    "source": "upload",
    "timestamp": "2025-05-08T16:30:00Z",
    "event_id": "evt123",
    "content_hash": "abcdef1234567890",
    "faces": [
      {
        "id": 1,
        "box": {
          "x_min": 100,
          "y_min": 200,
          "x_max": 300,
          "y_max": 400
        },
        "matches": [
          {
            "identity_id": 1,
            "name": "Person1",
            "confidence": 0.85
          }
        ]
      }
    ]
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Bild nicht gefunden"
  }
  ```

### Bild löschen

Löscht ein Bild aus dem System.

- **URL**: `/images/:id`
- **Methode**: `DELETE`
- **URL-Parameter**: `id` - ID des Bildes

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "message": "Bild erfolgreich gelöscht"
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Bild nicht gefunden"
  }
  ```

### Bild neu erkennen

Führt eine erneute Gesichtserkennung für ein bereits vorhandenes Bild durch.

- **URL**: `/images/:id/recognize`
- **Methode**: `POST`
- **URL-Parameter**: `id` - ID des Bildes

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "message": "Bild wurde zur erneuten Erkennung in die Warteschlange gestellt",
    "job_id": "job123"
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Bild nicht gefunden"
  }
  ```

## Identitäts-Endpunkte

### Identitäten auflisten

Ruft eine Liste aller Identitäten ab.

- **URL**: `/identities`
- **Methode**: `GET`

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "identities": [
      {
        "id": 1,
        "name": "Person1",
        "external_id": "person1_cf",
        "match_count": 25,
        "best_match_url": "/snapshots/best_match_1.jpg"
      }
    ]
  }
  ```

### Identität erstellen

Erstellt eine neue Identität.

- **URL**: `/identities`
- **Methode**: `POST`
- **Content-Type**: `application/json`

**Request-Body:**

```json
{
  "name": "NeueIdentität"
}
```

**Erfolgsantwort:**

- **Code**: 201 Created
- **Inhalt**:
  ```json
  {
    "id": 5,
    "name": "NeueIdentität",
    "external_id": "NeueIdentität",
    "created_at": "2025-05-08T16:40:00Z"
  }
  ```

### Identität abrufen

Ruft detaillierte Informationen zu einer bestimmten Identität ab.

- **URL**: `/identities/:id`
- **Methode**: `GET`
- **URL-Parameter**: `id` - ID der Identität

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "id": 1,
    "name": "Person1",
    "external_id": "person1_cf",
    "match_count": 25,
    "first_seen": "2025-01-01T10:00:00Z",
    "last_seen": "2025-05-08T15:30:00Z",
    "avg_confidence": 0.87,
    "matches": [
      {
        "id": 1,
        "image_id": 10,
        "face_id": 15,
        "confidence": 0.92,
        "timestamp": "2025-05-08T15:30:00Z",
        "image_path": "/snapshots/image10.jpg"
      }
    ]
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Identität nicht gefunden"
  }
  ```

### Identität löschen

Löscht eine Identität aus dem System.

- **URL**: `/identities/:id`
- **Methode**: `DELETE`
- **URL-Parameter**: `id` - ID der Identität

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "message": "Identität erfolgreich gelöscht"
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Identität nicht gefunden"
  }
  ```

### Identität umbenennen

Benennt eine existierende Identität um.

- **URL**: `/identities/:id/rename`
- **Methode**: `POST`
- **URL-Parameter**: `id` - ID der Identität
- **Content-Type**: `application/json`

**Request-Body:**

```json
{
  "name": "NeuerName"
}
```

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "id": 1,
    "name": "NeuerName",
    "external_id": "person1_cf",
    "updated_at": "2025-05-08T16:45:00Z"
  }
  ```

**Fehlerantwort:**

- **Code**: 404 Not Found
- **Inhalt**:
  ```json
  {
    "error": "Identität nicht gefunden"
  }
  ```

## System-Endpunkte

### System-Status abrufen

Ruft den aktuellen Status des Systems ab.

- **URL**: `/status`
- **Methode**: `GET`

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "status": "online",
    "version": "1.0.0",
    "system": {
      "cpu_usage": 23.5,
      "memory_usage": "512 MB",
      "go_routines": 25
    },
    "integrations": {
      "compreface": {
        "status": "connected",
        "subject_count": 15
      },
      "mqtt": {
        "status": "connected"
      },
      "frigate": {
        "status": "connected"
      }
    }
  }
  ```

### CompreFace-Synchronisation

Synchronisiert Identitäten mit CompreFace.

- **URL**: `/sync/compreface`
- **Methode**: `POST`

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "message": "Synchronisation mit CompreFace erfolgreich",
    "new_identities": 2,
    "updated_identities": 1
  }
  ```

**Fehlerantwort:**

- **Code**: 500 Internal Server Error
- **Inhalt**:
  ```json
  {
    "error": "Fehler bei der Synchronisation mit CompreFace"
  }
  ```

### Alle Trainingsdaten löschen

Löscht alle Trainingsdaten.

- **URL**: `/training/all`
- **Methode**: `DELETE`

**Erfolgsantwort:**

- **Code**: 200 OK
- **Inhalt**:
  ```json
  {
    "message": "Alle Trainingsdaten erfolgreich gelöscht"
  }
  ```

## Fehler-Antworten

Alle API-Endpunkte geben bei Fehlern standardisierte JSON-Antworten zurück:

```json
{
  "error": "Beschreibung des Fehlers",
  "code": "ERROR_CODE",
  "details": {}  // Optionale zusätzliche Details
}
```

## Authentifizierung

Derzeit bietet die API keine Authentifizierung. Es wird empfohlen, den Zugriff auf die API durch Netzwerkkonfiguration oder einen Reverse-Proxy mit Authentifizierung zu schützen, wenn die Anwendung öffentlich zugänglich ist.
