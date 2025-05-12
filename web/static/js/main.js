/**
 * Double-Take Go - Haupt-JavaScript-Datei
 */

document.addEventListener('DOMContentLoaded', function() {
    // Toast-Benachrichtigungen initialisieren
    initToasts();
    
    // Bildergalerie-Funktionen
    initImageGallery();
    
    // Server-Sent Events initialisieren
    initSSE();
    
    // Timeout für Erfolgs-/Fehlermeldungen
    setTimeout(function() {
        const alerts = document.querySelectorAll('.alert:not(.alert-permanent)');
        alerts.forEach(alert => {
            const bsAlert = new bootstrap.Alert(alert);
            bsAlert.close();
        });
    }, 5000);
});

/**
 * Toast-Benachrichtigungen initialisieren
 */
function initToasts() {
    const toastElList = document.querySelectorAll('.toast');
    toastElList.forEach(toastEl => {
        const toast = new bootstrap.Toast(toastEl);
        toast.show();
    });
}

/**
 * Bildergalerie-Funktionen initialisieren
 */
function initImageGallery() {
    // Lazy-Loading für Bilder aktivieren
    const lazyImages = document.querySelectorAll('img.lazy');
    if ('IntersectionObserver' in window) {
        const imageObserver = new IntersectionObserver((entries, observer) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    const img = entry.target;
                    img.src = img.dataset.src;
                    img.classList.remove('lazy');
                    imageObserver.unobserve(img);
                }
            });
        });

        lazyImages.forEach(img => {
            imageObserver.observe(img);
        });
    } else {
        // Fallback für ältere Browser
        lazyImages.forEach(img => {
            img.src = img.dataset.src;
            img.classList.remove('lazy');
        });
    }
    
    // Initialisiere Lightbox für Bilder, wenn vorhanden
    const galleryItems = document.querySelectorAll('[data-lightbox]');
    galleryItems.forEach(item => {
        item.addEventListener('click', function(e) {
            if (typeof bootstrap.Modal !== 'undefined') {
                e.preventDefault();
                
                const imgSrc = this.getAttribute('href') || this.getAttribute('src');
                
                // Prüfen, ob ein Modal bereits existiert oder eines erstellen
                let modal = document.getElementById('lightboxModal');
                
                if (!modal) {
                    // Modal erstellen, wenn keines existiert
                    modal = document.createElement('div');
                    modal.className = 'modal fade';
                    modal.id = 'lightboxModal';
                    modal.setAttribute('tabindex', '-1');
                    modal.setAttribute('aria-hidden', 'true');
                    
                    modal.innerHTML = `
                        <div class="modal-dialog modal-lg modal-dialog-centered">
                            <div class="modal-content">
                                <div class="modal-body p-0 text-center">
                                    <img src="${imgSrc}" class="img-fluid" alt="Lightbox Image">
                                </div>
                                <div class="modal-footer justify-content-center p-2">
                                    <button type="button" class="btn btn-sm btn-outline-secondary" data-bs-dismiss="modal">Schließen</button>
                                </div>
                            </div>
                        </div>
                    `;
                    
                    document.body.appendChild(modal);
                } else {
                    // Modal aktualisieren, wenn es bereits existiert
                    const modalImg = modal.querySelector('img');
                    modalImg.src = imgSrc;
                }
                
                // Modal anzeigen
                const bsModal = new bootstrap.Modal(modal);
                bsModal.show();
            }
        });
    });
}

/**
 * Aktualisiert die Bildergalerie mit einem neuen Bild
 * @param {Object} imageData - Daten des neuen Bildes
 */
function updateImageGallery(imageData) {
    const container = document.querySelector('[data-image-container]');
    if (!container) return;
    
    // Erstelle Toast-Benachrichtigung für das neue Bild
    createImageToast(imageData);
    
    // Überprüfen, ob eine Karte für dieses Bild bereits existiert
    const existingCard = document.querySelector(`[data-image-id="${imageData.id}"]`);
    
    // Badges generieren (für neue oder bestehende Karten)
    let badges = '';
    if (imageData.faces_count > 0) {
        badges += `<span class="badge bg-info">${imageData.faces_count} Gesichter</span> `;
    }
    
    if (imageData.matches && imageData.matches.length > 0) {
        badges += `<span class="badge bg-success">Erkannt</span>`;
    }
    
    // Zusätzliche Frigate-Badges für Frigate-Events
    if (imageData.source === 'frigate') {
        if (imageData.camera) {
            badges += ` <span class="badge bg-primary">${imageData.camera}</span>`;
        }
        if (imageData.label) {
            badges += ` <span class="badge bg-secondary">${imageData.label}</span>`;
        }
        if (imageData.zone) {
            badges += ` <span class="badge bg-warning text-dark">${imageData.zone}</span>`;
        }
    }
    
    // Korrekte Bildpfad-Generierung mit Fallback und Fehlerbehandlung
    let imagePath;
    if (imageData.snapshot_url && imageData.snapshot_url.trim() !== '') {
        imagePath = imageData.snapshot_url;
    } else if (imageData.file_path && imageData.file_path.trim() !== '') {
        // Stellen Sie sicher, dass der Pfad nicht doppelt mit /snapshots/ beginnt
        if (imageData.file_path.startsWith('/snapshots/')) {
            imagePath = imageData.file_path;
        } else {
            imagePath = '/snapshots/' + imageData.file_path;
        }
    } else {
        // Fallback auf Platzhalter-Bild, wenn kein gültiger Pfad vorhanden ist
        imagePath = '/static/img/no-image.png';
        console.warn(`Kein gültiger Bildpfad für Bild ID ${imageData.id} gefunden`);
    }
    
    if (existingCard) {
        // Bestehende Karte aktualisieren
        console.log(`Aktualisiere bestehende Karte für Bild ID ${imageData.id}`);
        
        // Bildquelle aktualisieren
        const img = existingCard.querySelector('img');
        if (img) {
            img.src = imagePath;
            // Wenn das Bild nicht geladen werden kann, auf Fallback zurücksetzen
            img.onerror = function() {
                this.onerror = null;
                this.src = '/static/img/no-image.png';
                console.log('Bild konnte nicht geladen werden, verwende Fallback-Bild');
            };
        }
        
        // Badges aktualisieren
        const badgesContainer = existingCard.querySelector('.mt-2');
        if (badgesContainer) {
            badgesContainer.innerHTML = badges;
        }
        
        // Wenn ein Update-Ereignis den Container wechseln muss (z.B. von 'no-faces' zu 'has-faces')
        const moveToContainer = (imageData.faces_count > 0) ? 
            document.querySelector('[data-image-container="has-faces"]') : 
            document.querySelector('[data-image-container="no-faces"]');
        
        if (moveToContainer && container !== moveToContainer) {
            console.log(`Verschiebe Karte zu anderem Container: ${imageData.faces_count > 0 ? 'has-faces' : 'no-faces'}`);
            const parentCol = existingCard.closest('.col-md-3');
            if (parentCol) {
                // Entferne aus dem aktuellen Container und füge zum neuen hinzu
                container.removeChild(parentCol);
                moveToContainer.insertBefore(parentCol, moveToContainer.firstChild);
            }
        }
    } else {
        // Neue Karte erstellen
        console.log(`Erstelle neue Karte für Bild ID ${imageData.id}`);
        const col = document.createElement('div');
        col.className = 'col-md-3 mb-4 fade-in';
        
        col.innerHTML = `
            <div class="card image-card" data-image-id="${imageData.id}">
                <a href="/images/${imageData.id}">
                    <img src="${imagePath}" class="image-thumbnail" alt="Bild" onerror="this.onerror=null; this.src='/static/img/no-image.png'; console.log('Bild konnte nicht geladen werden, verwende Fallback-Bild');">
                </a>
                <div class="card-body">
                    <div class="d-flex justify-content-between">
                        <h6 class="card-title">${imageData.source}</h6>
                        <small class="text-muted">${formatTime(imageData.timestamp)}</small>
                    </div>
                    <div class="mt-2">
                        ${badges}
                    </div>
                </div>
            </div>
        `;
        
        // Füge das neue Bild als erstes Element ein
        const firstChild = container.firstChild;
        if (firstChild) {
            container.insertBefore(col, firstChild);
        } else {
            container.appendChild(col);
        }
        
        // Entferne das letzte Element, wenn die Ansicht voll ist
        const items = container.querySelectorAll('.col-md-3');
        if (items.length > 12) {
            container.removeChild(items[items.length - 1]);
        }
    }
}

/**
 * Formatiert ein Datum für die Anzeige
 * @param {string} dateTime - Datums-String im ISO-Format
 * @returns {string} Formatiertes Datum
 */
function formatTime(dateTime) {
    if (!dateTime) return '';
    
    const date = new Date(dateTime);
    
    // Formatierung für "Heute", "Gestern" oder Datum
    const now = new Date();
    const yesterday = new Date(now);
    yesterday.setDate(now.getDate() - 1);
    
    const isToday = date.toDateString() === now.toDateString();
    const isYesterday = date.toDateString() === yesterday.toDateString();
    
    if (isToday) {
        return `Heute, ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
    } else if (isYesterday) {
        return `Gestern, ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
    } else {
        return `${date.getDate().toString().padStart(2, '0')}.${(date.getMonth() + 1).toString().padStart(2, '0')}.${date.getFullYear()}, ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
    }
}

/**
 * Initialisiert Server-Sent Events für Echtzeit-Updates
 */
function initSSE() {
    // Überprüfen, ob SSE vom Browser unterstützt wird
    if (typeof EventSource === 'undefined') {
        console.warn('Server-Sent Events werden vom Browser nicht unterstützt.');
        return;
    }
    
    // SSE-Verbindung herstellen
    const eventSource = new EventSource('/events');
    
    // Event-Handler für neue Bilder
    eventSource.addEventListener('message', function(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('SSE Nachricht empfangen:', data);
            
            // Bildergalerie aktualisieren, wenn sie existiert
            updateImageGallery(data);
        } catch (error) {
            console.error('Fehler beim Verarbeiten der SSE-Nachricht:', error);
        }
    });
    
    // Event-Handler für Verbindungsfehler
    eventSource.onerror = function() {
        console.warn('SSE-Verbindung unterbrochen. Versuche neu zu verbinden...');
    };
}

/**
 * Erstellt eine Toast-Benachrichtigung für ein neues Bild
 * @param {Object} imageData - Daten des neuen Bildes
 */
function createImageToast(imageData) {
    // Übersetzungen für Toast-Typen
    const toastTypeMap = {
        'frigate': 'Frigate',
        'upload': 'Upload',
        'webcam': 'Webcam',
        'other': 'Andere Quelle'
    };
    
    // Standard-Nachricht
    let title = toastTypeMap[imageData.source] || 'Neues Bild';
    let message = 'Ein neues Bild wurde erkannt.';
    let type = 'info';
    
    // Erweiterte Nachricht für Frigate-Events
    if (imageData.source === 'frigate') {
        // Verbesserte Toast-Meldung basierend auf verfügbaren Frigate-Metadaten
        if (imageData.label) {
            title = `${title}: ${imageData.label}`;
        }
        
        if (imageData.camera) {
            message = `Kamera: ${imageData.camera}`;
            if (imageData.zone) {
                message += `, Zone: ${imageData.zone}`;
            }
        }
        
        // Typ des Toasts basierend auf Event-Typ
        if (imageData.event_type === 'new') {
            type = 'warning';
        } else if (imageData.event_type === 'update') {
            type = 'info';
        }
        
        // Gesichtserkennung-Informationen hinzufügen
        if (imageData.faces_count > 0) {
            message += `, ${imageData.faces_count} Gesichter erkannt`;
            
            // Wenn Matches vorhanden sind, diese anzeigen
            if (imageData.matches && imageData.matches.length > 0) {
                const matchNames = imageData.matches.map(m => m.identity).join(', ');
                message += `: ${matchNames}`;
                type = 'success';
            }
        }
    }
    
    // Toast anzeigen
    showToast(title, message, type);
}
