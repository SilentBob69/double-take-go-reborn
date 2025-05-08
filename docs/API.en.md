# Double-Take Go API Documentation

This documentation describes the REST API of the Double-Take Go system, which can be used by external applications to interact with the system.

## Base URL

All API endpoints are relative to the server's base URL, by default:

```
http://localhost:3000/api
```

## API Overview

The API is divided into the following main areas:

- **Processing Endpoints**: For processing new images
- **Image Endpoints**: For managing and querying images
- **Identity Endpoints**: For managing detected persons/identities
- **System Endpoints**: For system functions and status

## Processing Endpoints

### Process Image

Uploads and processes a new image for face detection.

- **URL**: `/process/image`
- **Method**: `POST`
- **Content-Type**: `multipart/form-data`

**Form Parameters:**

| Parameter | Type | Description                  |
|-----------|------|------------------------------|
| file      | File | The image to be processed    |
| source    | Text | Source of the image (optional) |

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

**Error Response:**

- **Code**: 400 Bad Request
- **Content**:
  ```json
  {
    "error": "No file uploaded"
  }
  ```

### CompreFace Processing

Processes an image directly with CompreFace.

- **URL**: `/process/compreface`
- **Method**: `POST`
- **Content-Type**: `multipart/form-data`

**Form Parameters:**

| Parameter | Type | Description              |
|-----------|------|--------------------------|
| file      | File | The image to be processed |

**Success Response:**

- **Code**: 200 OK
- **Content**: CompreFace response in JSON format

## Image Endpoints

### List Images

Retrieves a list of all images, with optional filtering.

- **URL**: `/images`
- **Method**: `GET`

**Query Parameters:**

| Parameter  | Description                                      |
|------------|--------------------------------------------------|
| page       | Page number (default: 1)                         |
| limit      | Number of entries per page (default: 20)         |
| source     | Filter by source (e.g., "frigate", "upload")     |
| hasfaces   | Only images with/without faces ("yes"/"no")      |
| hasmatches | Only images with/without matches ("yes"/"no")    |
| daterange  | Time period ("today", "yesterday", "week", "month") |

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

### Get Image

Retrieves detailed information about a specific image.

- **URL**: `/images/:id`
- **Method**: `GET`
- **URL Parameters**: `id` - ID of the image

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Image not found"
  }
  ```

### Delete Image

Deletes an image from the system.

- **URL**: `/images/:id`
- **Method**: `DELETE`
- **URL Parameters**: `id` - ID of the image

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "message": "Image successfully deleted"
  }
  ```

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Image not found"
  }
  ```

### Recognize Image

Performs facial recognition for an existing image.

- **URL**: `/images/:id/recognize`
- **Method**: `POST`
- **URL Parameters**: `id` - ID of the image

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "message": "Image has been queued for recognition",
    "job_id": "job123"
  }
  ```

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Image not found"
  }
  ```

## Identity Endpoints

### List Identities

Retrieves a list of all identities.

- **URL**: `/identities`
- **Method**: `GET`

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

### Create Identity

Creates a new identity.

- **URL**: `/identities`
- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body:**

```json
{
  "name": "NewIdentity"
}
```

**Success Response:**

- **Code**: 201 Created
- **Content**:
  ```json
  {
    "id": 5,
    "name": "NewIdentity",
    "external_id": "NewIdentity",
    "created_at": "2025-05-08T16:40:00Z"
  }
  ```

### Get Identity

Retrieves detailed information about a specific identity.

- **URL**: `/identities/:id`
- **Method**: `GET`
- **URL Parameters**: `id` - ID of the identity

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Identity not found"
  }
  ```

### Delete Identity

Deletes an identity from the system.

- **URL**: `/identities/:id`
- **Method**: `DELETE`
- **URL Parameters**: `id` - ID of the identity

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "message": "Identity successfully deleted"
  }
  ```

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Identity not found"
  }
  ```

### Rename Identity

Renames an existing identity.

- **URL**: `/identities/:id/rename`
- **Method**: `POST`
- **URL Parameters**: `id` - ID of the identity
- **Content-Type**: `application/json`

**Request Body:**

```json
{
  "name": "NewName"
}
```

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "id": 1,
    "name": "NewName",
    "external_id": "person1_cf",
    "updated_at": "2025-05-08T16:45:00Z"
  }
  ```

**Error Response:**

- **Code**: 404 Not Found
- **Content**:
  ```json
  {
    "error": "Identity not found"
  }
  ```

## System Endpoints

### Get System Status

Retrieves the current status of the system.

- **URL**: `/status`
- **Method**: `GET`

**Success Response:**

- **Code**: 200 OK
- **Content**:
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

### CompreFace Synchronization

Synchronizes identities with CompreFace.

- **URL**: `/sync/compreface`
- **Method**: `POST`

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "message": "Synchronization with CompreFace successful",
    "new_identities": 2,
    "updated_identities": 1
  }
  ```

**Error Response:**

- **Code**: 500 Internal Server Error
- **Content**:
  ```json
  {
    "error": "Error synchronizing with CompreFace"
  }
  ```

### Delete All Training Data

Deletes all training data.

- **URL**: `/training/all`
- **Method**: `DELETE`

**Success Response:**

- **Code**: 200 OK
- **Content**:
  ```json
  {
    "message": "All training data successfully deleted"
  }
  ```

## Error Responses

All API endpoints return standardized JSON responses for errors:

```json
{
  "error": "Description of the error",
  "code": "ERROR_CODE",
  "details": {}  // Optional additional details
}
```

## Authentication

The API currently does not provide authentication. It is recommended to protect access to the API through network configuration or a reverse proxy with authentication if the application is publicly accessible.
