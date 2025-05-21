from fastapi import FastAPI, UploadFile, File, Form
from fastapi.responses import JSONResponse
import numpy as np
import cv2
import insightface
from insightface.app import FaceAnalysis
import base64
import os
import logging
import time
import uvicorn
from typing import Optional

# Logging konfigurieren
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("insightface-api")

# FastAPI-App initialisieren
app = FastAPI(title="InsightFace API")

# InsightFace Analyzer initialisieren
# Für GPU: providers=['CUDAExecutionProvider'] oder providers=['TensorrtExecutionProvider']
# Für CPU: providers=['CPUExecutionProvider']
providers = ['CPUExecutionProvider']  # Standard, wird durch ENV überschrieben
backend = os.environ.get('INFERENCE_BACKEND', 'onnx').lower()

if backend == 'trt' or backend == 'tensorrt':
    providers = ['TensorrtExecutionProvider']
elif backend == 'cuda':
    providers = ['CUDAExecutionProvider']
elif backend == 'ort' or backend == 'opencl':
    os.environ['USE_OPENCL'] = '1'
    providers = ['OpenCLExecutionProvider', 'CPUExecutionProvider']

logger.info(f"Initialisiere InsightFace mit Backend: {backend}, Providers: {providers}")

# Analyzer mit detection und recognition Modulen
analyzer = FaceAnalysis(providers=providers, allowed_modules=['detection', 'recognition'])
analyzer.prepare(ctx_id=0, det_size=(640, 640))

@app.get("/")
def read_root():
    return {"status": "ok", "message": "InsightFace API running"}

@app.get("/info")
def get_info():
    return {
        "status": "ok",
        "version": insightface.__version__,
        "backend": backend,
        "providers": providers
    }

@app.post("/detect")
async def detect_faces(file: UploadFile = File(...), 
                       min_face_size: Optional[int] = Form(20),
                       return_face_data: Optional[bool] = Form(False),
                       extract_embedding: Optional[bool] = Form(True)):
    try:
        start_time = time.time()
        
        # Bild einlesen
        contents = await file.read()
        nparr = np.frombuffer(contents, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        if img is None:
            return JSONResponse(status_code=400, content={"status": "error", "message": "Ungültiges Bildformat"})
        
        # Gesichter erkennen
        faces = analyzer.get(img)
        
        # Ergebnisse aufbereiten
        results = []
        for i, face in enumerate(faces):
            bbox = face.bbox.astype(int).tolist()
            confidence = float(face.det_score)
            
            result = {
                "bbox": bbox,
                "confidence": confidence,
            }
            
            # Optional: Einbettungsvektor (für Gesichtserkennung) zurückgeben
            if extract_embedding and hasattr(face, 'embedding') and face.embedding is not None:
                result["embedding"] = face.embedding.tolist()
            
            # Optional: Gesichtsdaten zurückgeben
            if return_face_data:
                x1, y1, x2, y2 = bbox
                face_img = img[y1:y2, x1:x2]
                if face_img.size > 0:
                    _, buffer = cv2.imencode('.jpg', face_img)
                    face_base64 = base64.b64encode(buffer).decode('utf-8')
                    result["face_data"] = face_base64
            
            results.append(result)
        
        process_time = time.time() - start_time
        
        return {
            "status": "ok",
            "faces_count": len(results),
            "faces": results,
            "process_time": process_time
        }
    except Exception as e:
        logger.error(f"Fehler bei der Gesichtserkennung: {str(e)}")
        return JSONResponse(status_code=500, content={
            "status": "error", 
            "message": f"Interner Serverfehler: {str(e)}"
        })

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 18081))
    uvicorn.run(app, host="0.0.0.0", port=port)
