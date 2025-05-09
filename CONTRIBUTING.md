# Beitragsrichtlinien

*Read this in [English](#contributing-guidelines)*

Vielen Dank für Ihr Interesse an Double-Take-Go-Reborn! Wir freuen uns sehr über jede Art von Beitrag, sei es durch Fehlerberichte, Funktionsvorschläge, Dokumentationsverbesserungen oder Code-Beiträge.

## Wie Sie beitragen können

Es gibt viele Möglichkeiten, zum Projekt beizutragen:

### 1. Feedback geben

Haben Sie Double-Take-Go-Reborn ausprobiert? Wir würden gerne hören, wie Ihre Erfahrung war:

- **Was hat gut funktioniert?**
- **Wo gab es Probleme?**
- **Haben Sie Vorschläge zur Verbesserung?**

Bitte teilen Sie Ihr Feedback in den [Discussions](https://github.com/SilentBob69/double-take-go-reborn/discussions/new?category=feedback) mit.

### 2. Fehler melden

Wenn Sie auf einen Fehler stoßen, erstellen Sie bitte einen [Issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new) mit folgenden Informationen:

- Klare Beschreibung des Problems
- Schritte zur Reproduktion
- Erwartetes und tatsächliches Verhalten
- Screenshots (falls hilfreich)
- Ihre Umgebung (Betriebssystem, Docker-Version, etc.)

### 3. Funktionen vorschlagen

Haben Sie eine Idee für eine neue Funktion? Erstellen Sie einen [Issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new) und beschreiben Sie:

- Was die Funktion tun soll
- Warum diese Funktion nützlich wäre
- Wie sie implementiert werden könnte (falls Sie Ideen haben)

### 4. Code beitragen

Code-Beiträge sind herzlich willkommen! So gehen Sie vor:

1. Forken Sie das Repository
2. Erstellen Sie einen Feature-Branch (`git checkout -b feature/amazing-feature`)
3. Committen Sie Ihre Änderungen (`git commit -m 'Add some amazing feature'`)
4. Pushen Sie den Branch (`git push origin feature/amazing-feature`)
5. Öffnen Sie einen Pull Request

### 5. Dokumentation verbessern

Klare Dokumentation ist entscheidend für die Benutzerfreundlichkeit:

- Korrigieren Sie Tippfehler oder unklare Erklärungen
- Fügen Sie Beispiele oder Tutorials hinzu
- Übersetzen Sie Dokumentation (DE/EN)

## Entwicklungsumgebung einrichten

Wir verwenden Docker für die Entwicklung. So richten Sie Ihre Umgebung ein:

```bash
# Entwicklungsumgebung starten
docker-compose -f docker-compose.dev.yml up -d

# In den Container einsteigen
docker exec -it double-take-go-reborn-go-dev-1 /bin/bash

# Anwendung im Container bauen
go build -o /app/bin/double-take /app/cmd/server/main.go

# Anwendung im Container starten
/app/bin/double-take /app/config/config.yaml
```

## Pull-Request-Prozess

1. Stellen Sie sicher, dass Ihr Code den Stilrichtlinien folgt
2. Aktualisieren Sie die README.md mit Details zu Änderungen, falls nötig
3. Ihre PR wird von mindestens einem Projektbetreuer geprüft werden

## Verhaltenskodex

Wir erwarten von allen Teilnehmern, dass sie respektvoll und inklusiv miteinander umgehen. Diskriminierung, Belästigung oder anderes unangemessenes Verhalten wird nicht toleriert. Konstruktives Feedback und Zusammenarbeit werden gefördert, während persönliche Angriffe vermieden werden sollten.

## Fragen?

Haben Sie Fragen zur Mitwirkung? Eröffnen Sie eine [Discussion](https://github.com/SilentBob69/double-take-go-reborn/discussions) oder kontaktieren Sie das Projektteam.

---

# Contributing Guidelines

*Lesen Sie dies auf [Deutsch](#beitragsrichtlinien)*

Thank you for your interest in Double-Take-Go-Reborn! We greatly appreciate any kind of contribution, whether it's bug reports, feature suggestions, documentation improvements, or code contributions.

## How to Contribute

There are many ways to contribute to the project:

### 1. Provide Feedback

Have you tried Double-Take-Go-Reborn? We'd love to hear about your experience:

- **What worked well?**
- **Where did you encounter issues?**
- **Do you have suggestions for improvement?**

Please share your feedback in the [Discussions](https://github.com/SilentBob69/double-take-go-reborn/discussions/new?category=feedback).

### 2. Report Bugs

If you find a bug, please create an [Issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new) with the following information:

- Clear description of the problem
- Steps to reproduce
- Expected and actual behavior
- Screenshots (if helpful)
- Your environment (OS, Docker version, etc.)

### 3. Suggest Features

Have an idea for a new feature? Create an [Issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new) describing:

- What the feature should do
- Why this feature would be useful
- How it might be implemented (if you have ideas)

### 4. Contribute Code

Code contributions are very welcome! Here's how to proceed:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### 5. Improve Documentation

Clear documentation is crucial for usability:

- Fix typos or unclear explanations
- Add examples or tutorials
- Translate documentation (DE/EN)

## Setting Up Development Environment

We use Docker for development. Set up your environment like this:

```bash
# Start development environment
docker-compose -f docker-compose.dev.yml up -d

# Enter the container
docker exec -it double-take-go-reborn-go-dev-1 /bin/bash

# Build the application in the container
go build -o /app/bin/double-take /app/cmd/server/main.go

# Start the application in the container
/app/bin/double-take /app/config/config.yaml
```

## Pull Request Process

1. Ensure your code follows the style guidelines
2. Update the README.md with details of changes if necessary
3. Your PR will be reviewed by at least one project maintainer

## Code of Conduct

We expect all participants to interact respectfully and inclusively. Discrimination, harassment, or other inappropriate behavior will not be tolerated. Constructive feedback and collaboration are encouraged, while personal attacks should be avoided.

## Questions?

Have questions about contributing? Open a [Discussion](https://github.com/SilentBob69/double-take-go-reborn/discussions) or contact the project team.
