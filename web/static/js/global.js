/**
 * Globale Funktionen für Double-Take Go Reborn
 */

// Funktion zum Speichern der Scrollposition und Ändern der Sprache
function changeLanguage(lang) {
    // Aktuelle Scrollposition speichern
    const scrollPosition = window.pageYOffset || document.documentElement.scrollTop;
    sessionStorage.setItem('scrollPosition', scrollPosition);
    
    // Aktuelle URL abrufen und Parameter hinzufügen oder aktualisieren
    const currentUrl = new URL(window.location.href);
    currentUrl.searchParams.set('lang', lang);
    
    // Zur neuen URL navigieren
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

// DOM-Inhalt geladen Event: Tooltips initialisieren
document.addEventListener('DOMContentLoaded', function() {
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
});

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

// Nach dem Laden der Seite die gespeicherte Scrollposition wiederherstellen
document.addEventListener('DOMContentLoaded', function() {
    // Gespeicherte Scrollposition abrufen
    const savedScrollPosition = sessionStorage.getItem('scrollPosition');
    
    if (savedScrollPosition) {
        // Nach kurzer Verzögerung scrollen, damit die Seite vollständig geladen ist
        setTimeout(function() {
            window.scrollTo(0, parseInt(savedScrollPosition));
            // Scrollposition aus dem Storage entfernen, um sie nicht mehrfach zu verwenden
            sessionStorage.removeItem('scrollPosition');
        }, 100);
    }
});
