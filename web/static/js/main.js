/**
 * Double-Take Go - Haupt-JavaScript-Datei
 */

document.addEventListener('DOMContentLoaded', function() {
    // Toast-Benachrichtigungen initialisieren
    initToasts();
    
    // Bildergalerie-Funktionen
    initImageGallery();
    
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
    
    // Erstelle ein neues Bild-Element
    const col = document.createElement('div');
    col.className = 'col-md-3 mb-4 fade-in';
    
    let badges = '';
    if (imageData.faceCount > 0) {
        badges += `<span class="badge bg-info">${imageData.faceCount} Gesichter</span> `;
    }
    
    if (imageData.hasMatches) {
        badges += `<span class="badge bg-success">Erkannt</span>`;
    }
    
    col.innerHTML = `
        <div class="card image-card">
            <a href="/images/${imageData.id}">
                <img src="/snapshots/${imageData.filePath}" class="image-thumbnail" alt="Bild">
            </a>
            <div class="card-body">
                <div class="d-flex justify-content-between">
                    <h6 class="card-title">${imageData.source}</h6>
                    <small class="text-muted">${formatTime(imageData.detectedAt)}</small>
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
