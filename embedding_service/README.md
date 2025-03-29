# Embedding Service

A FastAPI service that generates text embeddings using the `all-MiniLM-L6-v2` model from sentence-transformers.

## Features

- Text to embedding conversion
- Batch processing support
- REST API interface
- Docker support

## API Usage

### Generate Embeddings

**Endpoint:** `POST /embeddings`

**Request format:**
```json
{
  "texts": [
    "Your first text here",
    "Your second text here",
    "..."
  ]
}
```

**Example using curl:**
```bash
curl -X POST http://localhost:8000/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "texts": [
      "Hello world",
      "This is a test"
    ]
  }'
```

For multiple texts with better formatting:
```bash
curl -X POST http://localhost:8000/embeddings \
  -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "texts": [
    "Hello world",
    "This is a test",
    "Another example text"
  ]
}
EOF
```

## Running Locally

1. Install dependencies:
```bash
pip install -r requirements.txt
```

2. Start the server:
```bash
python main.py
```

The service will be available at `http://localhost:8000`

## Docker

Build and run using Docker:

```bash
docker build -t embedding-service .
docker run -p 8000:8000 embedding-service
```

Or pull from GitHub Container Registry:

```bash
docker pull ghcr.io/korjavin/ragtgbot:latest
docker run -p 8000:8000 ghcr.io/korjavin/ragtgbot:latest
```