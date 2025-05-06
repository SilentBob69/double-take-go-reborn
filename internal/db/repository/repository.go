package repository

import (
	"errors"

	"double-take-go-reborn/internal/core/models"

	"gorm.io/gorm"
)

// Repository definiert die Schnittstelle für die Datenbank-Operationen
type Repository interface {
	// Image-Methoden
	GetImageByID(id uint) (*models.Image, error)
	GetImages(limit, offset int) ([]models.Image, int64, error)
	SaveImage(image *models.Image) error
	DeleteImage(id uint) error

	// Face-Methoden
	GetFaceByID(id uint) (*models.Face, error)
	GetFacesByImageID(imageID uint) ([]models.Face, error)
	SaveFace(face *models.Face) error
	DeleteFace(id uint) error

	// Identity-Methoden
	GetIdentityByID(id uint) (*models.Identity, error)
	GetIdentities() ([]models.Identity, error)
	SaveIdentity(identity *models.Identity) error
	DeleteIdentity(id uint) error
	FindIdentityByExternalID(externalID string) (*models.Identity, error)

	// Match-Methoden
	GetMatchByID(id uint) (*models.Match, error)
	GetMatchesByFaceID(faceID uint) ([]models.Match, error)
	GetMatchesByIdentityID(identityID uint) ([]models.Match, error)
	SaveMatch(match *models.Match) error
	DeleteMatch(id uint) error

	// Statistik-Methoden
	GetStatistics() (models.Statistics, error)
}

// SQLiteRepository implementiert die Repository-Schnittstelle für SQLite
type SQLiteRepository struct {
	db *gorm.DB
}

// NewSQLiteRepository erstellt eine neue SQLite-Repository-Instanz
func NewSQLiteRepository(db *gorm.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Image-Methoden

// GetImageByID holt ein Bild anhand seiner ID
func (r *SQLiteRepository) GetImageByID(id uint) (*models.Image, error) {
	var image models.Image
	result := r.db.First(&image, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &image, nil
}

// GetImages holt Bilder mit Pagination
func (r *SQLiteRepository) GetImages(limit, offset int) ([]models.Image, int64, error) {
	var images []models.Image
	var total int64

	r.db.Model(&models.Image{}).Count(&total)
	result := r.db.Order("detected_at DESC").Limit(limit).Offset(offset).Find(&images)
	
	if result.Error != nil {
		return nil, 0, result.Error
	}
	
	return images, total, nil
}

// SaveImage speichert ein Bild
func (r *SQLiteRepository) SaveImage(image *models.Image) error {
	return r.db.Save(image).Error
}

// DeleteImage löscht ein Bild
func (r *SQLiteRepository) DeleteImage(id uint) error {
	return r.db.Delete(&models.Image{}, id).Error
}

// Face-Methoden

// GetFaceByID holt ein Gesicht anhand seiner ID
func (r *SQLiteRepository) GetFaceByID(id uint) (*models.Face, error) {
	var face models.Face
	result := r.db.First(&face, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &face, nil
}

// GetFacesByImageID holt alle Gesichter für ein bestimmtes Bild
func (r *SQLiteRepository) GetFacesByImageID(imageID uint) ([]models.Face, error) {
	var faces []models.Face
	result := r.db.Where("image_id = ?", imageID).Find(&faces)
	if result.Error != nil {
		return nil, result.Error
	}
	return faces, nil
}

// SaveFace speichert ein Gesicht
func (r *SQLiteRepository) SaveFace(face *models.Face) error {
	return r.db.Save(face).Error
}

// DeleteFace löscht ein Gesicht
func (r *SQLiteRepository) DeleteFace(id uint) error {
	return r.db.Delete(&models.Face{}, id).Error
}

// Identity-Methoden

// GetIdentityByID holt eine Identität anhand ihrer ID
func (r *SQLiteRepository) GetIdentityByID(id uint) (*models.Identity, error) {
	var identity models.Identity
	result := r.db.First(&identity, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &identity, nil
}

// GetIdentities holt alle Identitäten
func (r *SQLiteRepository) GetIdentities() ([]models.Identity, error) {
	var identities []models.Identity
	result := r.db.Find(&identities)
	if result.Error != nil {
		return nil, result.Error
	}
	return identities, nil
}

// SaveIdentity speichert eine Identität
func (r *SQLiteRepository) SaveIdentity(identity *models.Identity) error {
	return r.db.Save(identity).Error
}

// DeleteIdentity löscht eine Identität
func (r *SQLiteRepository) DeleteIdentity(id uint) error {
	return r.db.Delete(&models.Identity{}, id).Error
}

// FindIdentityByExternalID sucht eine Identität anhand der externen ID
func (r *SQLiteRepository) FindIdentityByExternalID(externalID string) (*models.Identity, error) {
	var identity models.Identity
	result := r.db.Where("external_id = ?", externalID).First(&identity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &identity, nil
}

// Match-Methoden

// GetMatchByID holt einen Match anhand seiner ID
func (r *SQLiteRepository) GetMatchByID(id uint) (*models.Match, error) {
	var match models.Match
	result := r.db.First(&match, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &match, nil
}

// GetMatchesByFaceID holt alle Matches für ein bestimmtes Gesicht
func (r *SQLiteRepository) GetMatchesByFaceID(faceID uint) ([]models.Match, error) {
	var matches []models.Match
	result := r.db.Where("face_id = ?", faceID).Find(&matches)
	if result.Error != nil {
		return nil, result.Error
	}
	return matches, nil
}

// GetMatchesByIdentityID holt alle Matches für eine bestimmte Identität
func (r *SQLiteRepository) GetMatchesByIdentityID(identityID uint) ([]models.Match, error) {
	var matches []models.Match
	result := r.db.Where("identity_id = ?", identityID).Find(&matches)
	if result.Error != nil {
		return nil, result.Error
	}
	return matches, nil
}

// SaveMatch speichert einen Match
func (r *SQLiteRepository) SaveMatch(match *models.Match) error {
	return r.db.Save(match).Error
}

// DeleteMatch löscht einen Match
func (r *SQLiteRepository) DeleteMatch(id uint) error {
	return r.db.Delete(&models.Match{}, id).Error
}

// Statistik-Methoden

// GetStatistics gibt Statistiken über die gespeicherten Daten zurück
func (r *SQLiteRepository) GetStatistics() (models.Statistics, error) {
	var stats models.Statistics
	
	// Zähle Bilder
	if err := r.db.Model(&models.Image{}).Count(&stats.TotalImages).Error; err != nil {
		return stats, err
	}
	
	// Zähle Gesichter
	if err := r.db.Model(&models.Face{}).Count(&stats.TotalFaces).Error; err != nil {
		return stats, err
	}
	
	// Zähle Identitäten
	if err := r.db.Model(&models.Identity{}).Count(&stats.IdentityCount).Error; err != nil {
		return stats, err
	}
	
	// Zähle identifizierte Gesichter (Matches)
	if err := r.db.Model(&models.Match{}).
		Select("COUNT(DISTINCT face_id)").
		Count(&stats.IdentifiedFaces).Error; err != nil {
		return stats, err
	}
	
	// Berechne unbekannte Gesichter
	stats.UnknownFaces = stats.TotalFaces - stats.IdentifiedFaces
	
	// Ermittle das neueste Bild
	var latestImage models.Image
	if err := r.db.Order("timestamp DESC").First(&latestImage).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return stats, err
		}
	} else {
		stats.LatestImage = latestImage.Timestamp
	}
	
	// Hole die letzten 5 Bilder für RecentDetections
	if err := r.db.Order("timestamp DESC").Limit(5).Find(&stats.RecentDetections).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return stats, err
		}
	}
	
	return stats, nil
}
