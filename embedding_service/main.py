from fastapi import FastAPI, Request
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer
import uvicorn
import json

app = FastAPI()

# Load the Sentence Transformer model
model_name = "all-MiniLM-L6-v2"
model = SentenceTransformer(model_name)

class TextList(BaseModel):
    texts: list[str]

@app.post("/embeddings")
async def get_embeddings(text_list: TextList):
    """
    Generates embeddings for a list of texts using the SentenceTransformer model.
    """
    try:
        embeddings = model.encode(text_list.texts)
        return json.dumps(embeddings.tolist())
    except Exception as e:
        return {"error": str(e)}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)