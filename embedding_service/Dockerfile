# syntax=docker/dockerfile:1.4
# Stage 1: Builder - install OS build tools and Python deps
FROM python:3.10-slim AS builder
WORKDIR /app

# Install build tools (if needed for compiling deps like numpy, tokenizers, etc.)
RUN apt-get update && apt-get install -y --no-install-recommends build-essential

COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip \
pip install -r requirements.txt  # Install all Python dependencies

# Stage 2: Final image - copy deps from builder and add app code
FROM python:3.10-slim
WORKDIR /app

# Copy installed packages from builder (this brings in all libs installed by pip)
COPY --from=builder /usr/local /usr/local

# Copy application code
COPY . .

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
