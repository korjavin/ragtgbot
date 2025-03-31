# syntax=docker/dockerfile:1.4
# Stage 1: Builder - install OS build tools and Python deps
FROM python:3.10-slim AS builder
WORKDIR /app

# Install build tools (if needed for compiling deps like numpy, tokenizers, etc.)
RUN apt-get update && apt-get install -y --no-install-recommends build-essential

COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip \
pip install -r requirements.txt  # Install all Python dependencies

# Set up model directory
RUN mkdir -p /app/models

# Pre-download the model during build
RUN python -c "from sentence_transformers import SentenceTransformer; \
    model = SentenceTransformer('distiluse-base-multilingual-cased-v1'); \
    model.save('/app/models/distiluse-base-multilingual-cased-v1')"


# Stage 2: Final image - copy deps from builder and add app code
FROM python:3.10-slim
WORKDIR /app

# Copy installed packages from builder (this brings in all libs installed by pip)
COPY --from=builder /usr/local /usr/local

# Copy model
COPY --from=builder /app/models /app/models

# Set environment variable for local model path
ENV LOCAL_MODEL_PATH=/app/models/distiluse-base-multilingual-cased-v1

# Copy application code
COPY . .

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
