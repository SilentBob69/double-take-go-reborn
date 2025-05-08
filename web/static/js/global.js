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
