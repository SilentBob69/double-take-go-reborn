FROM python:3.9-slim

# Arbeitsverzeichnis setzen
WORKDIR /app

# Systemabhängigkeiten installieren
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libgl1-mesa-glx \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Python-Abhängigkeiten installieren
RUN pip install --no-cache-dir \
    fastapi==0.95.0 \
    uvicorn==0.21.1 \
    numpy==1.24.2 \
    scipy==1.10.1 \
    opencv-python-headless==4.7.0.72 \
    onnx==1.13.1 \
    onnxruntime==1.14.1 \
    insightface==0.7.3 \
    python-multipart==0.0.6 \
    pillow==9.5.0

# Ports
EXPOSE 18081

# App kopieren
COPY app.py /app/

# Gesichtsmodelle herunterladen und warm-up
RUN python -c "import insightface; from insightface.app import FaceAnalysis; \
    analyzer = FaceAnalysis(allowed_modules=['detection', 'recognition']); \
    analyzer.prepare(ctx_id=0, det_size=(640, 640))"

# Umgebungsvariablen
ENV PORT=18081
ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/app

# Uvicorn starten
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "18081"]
