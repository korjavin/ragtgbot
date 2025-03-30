from fastapi import FastAPI, Request
from pydantic import BaseModel
import uvicorn
import json
import logging
import os

app = FastAPI()

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize model variable and configuration
model = None
MODEL_NAME = "BAAI/bge-multilingual-gemma2"
LOCAL_MODEL_PATH = os.environ.get("LOCAL_MODEL_PATH", "/app/models/bge-multilingual-gemma2")

try:
    # Import SentenceTransformer after logging is set up to catch import errors
    from sentence_transformers import SentenceTransformer
    import torch
    
    # First try loading from local path
    if os.path.exists(LOCAL_MODEL_PATH):
        logger.info(f"Loading model from local path: {LOCAL_MODEL_PATH}")
        model = SentenceTransformer(LOCAL_MODEL_PATH, model_kwargs={"torch_dtype": torch.float16})
        logger.info("Model loaded successfully from local path")
    else:
        # Fallback to downloading from HuggingFace
        logger.info(f"Local model not found. Downloading model: {MODEL_NAME}")
        model = SentenceTransformer(MODEL_NAME, model_kwargs={"torch_dtype": torch.float16})
        logger.info("Model downloaded and loaded successfully")
except Exception as e:
    logger.error(f"Error loading model: {str(e)}")

class TextList(BaseModel):
    texts: list[str]

@app.post("/embeddings")
async def get_embeddings(text_list: TextList):
    """
    Generates embeddings for a list of texts using the SentenceTransformer model.
    """
    if model is None:
        return {"error": "Model not initialized. Check server logs."}
    
    try:
        embeddings = model.encode(text_list.texts)
        return json.dumps(embeddings.tolist())
    except Exception as e:
        logger.error(f"Error generating embeddings: {str(e)}")
        return {"error": str(e)}

@app.get("/health")
async def health_check():
    """Health check endpoint to verify the service is running."""
    return {"status": "ok", "model_loaded": model is not None}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)