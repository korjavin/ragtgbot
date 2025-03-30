from fastapi import FastAPI, Request
from pydantic import BaseModel
import uvicorn
import json
import logging

app = FastAPI()

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize model variable
model = None

try:
    # Import SentenceTransformer after logging is set up to catch import errors
    from sentence_transformers import SentenceTransformer
    
    model_name = "BAAI/bge-multilingual-gemma2"  
    #model_name = "all-MiniLM-L6-v2"
    logger.info(f"Loading model: {model_name}")
    model = SentenceTransformer(model_name)
    logger.info("Model loaded successfully")
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