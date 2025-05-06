#!/bin/bash

echo "Building and starting Double-Take-Go..."
docker-compose build
docker-compose up -d

echo "Double-Take-Go sollte nun unter http://localhost:3000 erreichbar sein."
echo "Server-Logs anzeigen mit: docker-compose logs -f"
echo "Server stoppen mit: docker-compose down"
