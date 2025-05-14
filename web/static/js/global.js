/**
 * Globale Funktionen für Double-Take Go Reborn
 */

// Funktion zum Speichern der Scrollposition und Ändern der Sprache
function changeLanguage(lang) {
    // Aktuelle Scrollposition speichern
    const scrollPosition = window.pageYOffset || document.documentElement.scrollTop;
    sessionStorage.setItem('scrollPosition', scrollPosition);
    
    // Sprache in Cookie speichern (30 Tage gültig)
    const expirationDate = new Date();
    expirationDate.setDate(expirationDate.getDate() + 30);
    document.cookie = `lang=${lang}; expires=${expirationDate.toUTCString()}; path=/; SameSite=Lax`;
    
    // Aktuelle URL abrufen und Parameter hinzufügen oder aktualisieren
    const currentUrl = new URL(window.location.href);
    currentUrl.searchParams.set('lang', lang);
    
    // Zur neuen URL navigieren (mit vollständigem Neuladen)
    window.location.href = currentUrl.toString();
}

// Tooltips initialisieren
function initTooltips() {
    // Alle data-bs-toggle="tooltip" Elemente finden und initialisieren
    const tooltipTriggerList = document.querySelectorAll('[data-bs-toggle="tooltip"]');
    if (tooltipTriggerList.length > 0) {
        [...tooltipTriggerList].map(tooltipTriggerEl => new bootstrap.Tooltip(tooltipTriggerEl));
    }
    
    // MutationObserver für dynamisch hinzugefügte Elemente
    const observer = new MutationObserver(mutations => {
        mutations.forEach(mutation => {
            if (mutation.addedNodes && mutation.addedNodes.length > 0) {
                mutation.addedNodes.forEach(node => {
                    if (node.nodeType === 1) { // ELEMENT_NODE
                        const tooltips = node.querySelectorAll('[data-bs-toggle="tooltip"]');
                        tooltips.forEach(el => {
                            new bootstrap.Tooltip(el);
                        });
                    }
                });
            }
        });
    });
    
    // Observer auf gesamtes Dokument anwenden
    observer.observe(document.body, { childList: true, subtree: true });
}

// Toast-Benachrichtigungsfunktion
function showToast(title, message, type) {
    // Bestehenden Toast-Container finden oder erstellen
    let toastContainer = document.querySelector('.toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.className = 'toast-container position-fixed top-0 end-0 p-3';
        toastContainer.style.zIndex = '1050';
        document.body.appendChild(toastContainer);
    }
    
    // Eindeutige ID für diesen Toast generieren
    const toastId = 'toast-' + Date.now();
    
    // Toast-Element erstellen
    const toastElement = document.createElement('div');
    toastElement.id = toastId;
    toastElement.className = `toast align-items-center text-white border-0 toast-${type}`;
    toastElement.setAttribute('role', 'alert');
    toastElement.setAttribute('aria-live', 'assertive');
    toastElement.setAttribute('aria-atomic', 'true');
    
    // Toast-Inhalt erstellen
    toastElement.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">
                <strong>${title}</strong> ${message}
            </div>
            <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
        </div>
    `;
    
    // Toast zum Container hinzufügen
    toastContainer.appendChild(toastElement);
    
    // Toast initialisieren und anzeigen
    const toast = new bootstrap.Toast(toastElement, {
        delay: 3000,
        autohide: true
    });
    toast.show();
    
    // Toast nach dem Ausblenden entfernen
    toastElement.addEventListener('hidden.bs.toast', function () {
        this.remove();
    });
}

/**
 * Startet den Prozess zum Anlernen von CompreFace mit einem erkannten Gesicht
 * @param {string} imageId - ID des Bildes mit dem erkannten Gesicht
 */
function trainCompreFaceWithImage(imageId) {
    console.log(`Starte CompreFace-Training mit Bild ${imageId}`);
    
    // Prüfen, ob das Bild Gesichter enthält
    fetch(`/api/images/${imageId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('Bilddaten geladen:', data);
            
            if (!data.faces || data.faces.length === 0) {
                showToast('Hinweis', 'Dieses Bild enthält keine erkannten Gesichter für das Training', 'warning');
                return;
            }
            
            // Lade die Liste der Identitäten für das Modal
            fetch('/api/identities')
                .then(response => response.json())
                .then(identities => {
                    // Modal vorbereiten und anzeigen
                    const modalHtml = `
                        <div class="modal fade" id="compreFaceTrainImageModal" tabindex="-1" aria-hidden="true">
                            <div class="modal-dialog">
                                <div class="modal-content">
                                    <div class="modal-header">
                                        <h5 class="modal-title">${document.documentElement.lang === 'de' ? 'CompreFace anlernen' : 'Train CompreFace'}</h5>
                                        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                                    </div>
                                    <div class="modal-body">
                                        <div class="row mb-3">
                                            <div class="col-md-6">
                                                <img src="/snapshots/${data.filePath}" class="img-fluid img-thumbnail" alt="Trainingsbild">
                                            </div>
                                            <div class="col-md-6">
                                                <form id="trainCompreFaceForm">
                                                    <input type="hidden" name="image_id" value="${imageId}">
                                                    <div class="mb-3">
                                                        <label for="identity_select" class="form-label">${document.documentElement.lang === 'de' ? 'Identität auswählen' : 'Select identity'}</label>
                                                        <select class="form-select" id="identity_select" name="identity_id" required>
                                                            <option value="" disabled selected>${document.documentElement.lang === 'de' ? 'Bitte wählen Sie eine Identität' : 'Please select an identity'}</option>
                                                            ${identities.map(identity => `<option value="${identity.ID}">${identity.Name}</option>`).join('')}
                                                        </select>
                                                    </div>
                                                    <div class="mb-3">
                                                        <label for="face_select" class="form-label">${document.documentElement.lang === 'de' ? 'Gesicht auswählen' : 'Select face'}</label>
                                                        <select class="form-select" id="face_select" name="face_id" required>
                                                            <option value="" disabled selected>${document.documentElement.lang === 'de' ? 'Bitte wählen Sie ein Gesicht' : 'Please select a face'}</option>
                                                            ${data.faces.map((face, index) => {
                                                                let label = `Gesicht ${index + 1}`;
                                                                if (face.matches && face.matches.length > 0) {
                                                                    label += ` (${face.matches[0].identity.name}, ${Math.round(face.matches[0].confidence * 100)}%)`;
                                                                }
                                                                return `<option value="${face.ID}">${label}</option>`;
                                                            }).join('')}
                                                        </select>
                                                    </div>
                                                </form>
                                            </div>
                                        </div>
                                    </div>
                                    <div class="modal-footer">
                                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">${document.documentElement.lang === 'de' ? 'Abbrechen' : 'Cancel'}</button>
                                        <button type="button" class="btn btn-primary" id="submitTrainCompreFace">
                                            <i class="bi bi-mortarboard me-1"></i> ${document.documentElement.lang === 'de' ? 'Anlernen' : 'Train'}
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    `;
                    
                    // Existierendes Modal entfernen, falls vorhanden
                    const existingModal = document.getElementById('compreFaceTrainImageModal');
                    if (existingModal) {
                        existingModal.remove();
                    }
                    
                    // Modal zum DOM hinzufügen
                    document.body.insertAdjacentHTML('beforeend', modalHtml);
                    
                    // Modal initialisieren und anzeigen
                    const modalElement = document.getElementById('compreFaceTrainImageModal');
                    const modal = new bootstrap.Modal(modalElement);
                    modal.show();
                    
                    // Submit-Event für das Anlernen
                    document.getElementById('submitTrainCompreFace').addEventListener('click', function() {
                        const form = document.getElementById('trainCompreFaceForm');
                        const identityId = form.querySelector('#identity_select').value;
                        const faceId = form.querySelector('#face_select').value;
                        
                        if (!identityId) {
                            showToast('Fehler', document.documentElement.lang === 'de' ? 'Bitte wählen Sie eine Identität aus' : 'Please select an identity', 'danger');
                            return;
                        }
                        
                        if (!faceId) {
                            showToast('Fehler', document.documentElement.lang === 'de' ? 'Bitte wählen Sie ein Gesicht aus' : 'Please select a face', 'danger');
                            return;
                        }
                        
                        // Submit via AJAX
                        fetch(`/api/faces/${faceId}/train-compreface`, {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                identity_id: identityId,
                                image_id: imageId
                            })
                        })
                        .then(response => {
                            if (!response.ok) {
                                throw new Error(`HTTP error! Status: ${response.status}`);
                            }
                            return response.json();
                        })
                        .then(data => {
                            if (data.success) {
                                // Erfolgsmeldung anzeigen
                                showToast('Erfolg', data.message, 'success');
                                // Modal schließen
                                modal.hide();
                            } else {
                                showToast('Fehler', data.error || 'Unbekannter Fehler', 'danger');
                            }
                        })
                        .catch(error => {
                            console.error('Error:', error);
                            showToast('Fehler', document.documentElement.lang === 'de' ? 'Serverfehler beim Verarbeiten der Anfrage' : 'Server error processing the request', 'danger');
                        });
                    });
                });
        })
        .catch(error => {
            console.error('Error:', error);
            showToast('Fehler', 'Fehler beim Laden der Bilddaten', 'danger');
        });
}

/**
 * Initialisiert die Event-Listener für die CompreFace-Training-Buttons
 */
function initCompreFaceTrainingButtons() {
    console.log('DEBUG: Initialisiere CompreFace-Training-Buttons');
    
    // Detaillierte Debug-Ausgabe für die Button-Selektoren
    console.log('DEBUG: Button-Suche beginnt');
    debugElement('.train-compreface-btn');
    debugElement('[data-action="train-compreface"]');
    
    // Direkte Methode für die Button-Initialisierung
    document.querySelectorAll('.train-compreface-btn, [data-action="train-compreface"]').forEach(button => {
        // Sichtbarkeit verbessern
        button.style.zIndex = '1000';
        button.style.position = 'relative';
        button.style.pointerEvents = 'auto';
        button.style.cursor = 'pointer';
        
        // Hervorhebung für Debug-Zwecke
        button.style.border = '2px solid red';
        
        console.log('DEBUG: Button gefunden und Event-Listener wird hinzugefügt:', button);
        console.log('DEBUG: Button dataset:', button.dataset);
        
        // Event-Listener mit capture und useCapture für maximale Kompatibilität
        button.addEventListener('click', function(e) {
            console.log('DEBUG: CompreFace-Button wurde geklickt!');
            e.preventDefault();
            e.stopPropagation();
            
            const imageId = this.dataset.imageId;
            console.log(`DEBUG: Bild-ID aus Dataset: ${imageId}`);
            
            if (imageId) {
                console.log(`DEBUG: Rufe trainCompreFaceWithImage mit ID ${imageId} auf`);
                trainCompreFaceWithImage(imageId);
            } else {
                console.error('DEBUG: Keine Bild-ID gefunden!');
            }
        }, true);
        
        // Zusätzlicher direkter onclick-Handler als Fallback
        button.onclick = function(e) {
            console.log('DEBUG: Button onclick ausgelöst');
            e.preventDefault();
            const imageId = this.dataset.imageId;
            if (imageId) {
                trainCompreFaceWithImage(imageId);
            }
        };
    });
}

// Debug-Funktion zum Ausgeben von Informationen zu DOM-Elementen
function debugElement(selector) {
    const elements = document.querySelectorAll(selector);
    console.log(`Gefundene Elemente mit Selektor "${selector}": ${elements.length}`);
    elements.forEach((el, index) => {
        console.log(`Element ${index + 1}:`, el);
        console.log('  - Attribute:', Object.fromEntries([...el.attributes].map(attr => [attr.name, attr.value])));
        console.log('  - Dataset:', el.dataset);
        console.log('  - Styles:', getComputedStyle(el));
        console.log('  - isVisible:', el.offsetParent !== null);
    });
}

// DOM-Inhalt geladen Event: Tooltips initialisieren und Event-Listener registrieren
document.addEventListener('DOMContentLoaded', function() {
    console.log('DOM vollständig geladen - Initialisiere Eventlistener und Funktionen');
    // Scrollposition wiederherstellen
    const scrollPosition = sessionStorage.getItem('scrollPosition');
    if (scrollPosition) {
        window.scrollTo(0, parseInt(scrollPosition));
        sessionStorage.removeItem('scrollPosition');
    }
    
    // Tooltips initialisieren, wenn Bootstrap geladen ist
    if (typeof bootstrap !== 'undefined') {
        console.log('Initialisiere Tooltips...');
        initTooltips();
    } else {
        console.warn('Bootstrap ist nicht geladen, Tooltips werden nicht initialisiert');
    }
    
    // Event-Delegation für alle Button-Klicks auf Dokumentebene
    document.addEventListener('click', function(e) {
        // Event-Gruppe anzeigen Buttons
        if (e.target.closest('.view-event-btn')) {
            e.preventDefault();
            const button = e.target.closest('.view-event-btn');
            const eventId = button.getAttribute('data-event-id');
            if (eventId) {
                window.location.href = `/events/${eventId}`;
            }
        }
        
        // CompreFace-Training Buttons
        if (e.target.closest('.dt-train-compreface') || e.target.closest('[data-action="train-compreface"]')) {
            e.preventDefault();
            const button = e.target.closest('.dt-train-compreface') || e.target.closest('[data-action="train-compreface"]');
            const imageId = button.getAttribute('data-image-id');
            console.log('CompreFace-Anlernen-Button geklickt. Bild-ID:', imageId);
            if (imageId) {
                trainCompreFaceWithImage(imageId);
            }
        }
    });
});
