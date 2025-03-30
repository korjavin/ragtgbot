# RAGTGBOT - Retrieval-Augmented Generation Telegram Bot

![Go Tests](https://github.com/korjavin/ragtgbot/actions/workflows/go-test.yml/badge.svg)
![Go Checks](https://github.com/korjavin/ragtgbot/actions/workflows/go-checks.yml/badge.svg)

A Telegram bot that uses vector embeddings and RAG to provide context-aware responses.

## Features

- Listens to and stores chat messages in a vector database
- Uses embeddings to find similar messages when questions are asked
- Generates responses using OpenAI models with chat history context
- Supports uploading Telegram chat backup files

## Overview

This project implements a Telegram bot that:

1. **Stores all messages** it receives in a vector database (Qdrant) with their embeddings
2. **Retrieves semantically similar messages** when queried
3. **Generates intelligent responses** using OpenAI's GPT models based on the retrieved context
4. **Stays up-to-date** with the latest conversations in real-time

The bot uses a Retrieval-Augmented Generation (RAG) approach, which enhances large language model responses with relevant information from a knowledge base - in this case, the chat history.

## Components

The project consists of three main components:

### 1. Telegram Bot (cmd/tgbot)

The core bot that:
- Interacts with users through Telegram
- Processes incoming messages and queries
- Stores messages in the vector database
- Retrieves relevant messages when queried
- Calls OpenAI API to generate responses
- Presents both AI-generated answers and relevant messages to users

### 2. Embedding Service (embedding_service)

A FastAPI service that:
- Generates text embeddings using the `https://huggingface.co/sentence-transformers/distiluse-base-multilingual-cased-v1` model
- Provides a REST API for embedding generation
- Supports batch processing of texts
- Runs as a separate microservice

### 3. Backup Uploader (cmd/uploadbackup)

A utility tool that:
- Parses Telegram group backups in JSON format
- Extracts text from each message
- Calculates embeddings using the embedding service
- Saves message data to the Qdrant vector database

## How It Works

1. **Message Storage**:
   - When the bot receives a regular message, it generates embeddings for the text
   - The message text, username, and embeddings are stored in Qdrant
   - The bot intelligently ignores its own messages to prevent redundant storage

2. **Query Processing**:
   - When the bot is mentioned with a query, it generates embeddings for the query
   - It searches the vector database for the top 10 semantically similar messages
   - It constructs a prompt for OpenAI using these messages
   - It calls the OpenAI API to generate a response
   - It returns both the AI-generated answer and the top 5 most relevant messages

3. **Historical Data Import**:
   - The backup uploader tool can be used to import historical chat data
   - This allows the bot to have context from conversations that happened before it was added to a group

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Telegram Bot Token (from BotFather)
- OpenAI API Key

### Environment Variables

Required:
- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token from BotFather
- `OPENAI_API_KEY`: Your OpenAI API key

Optional:
- `TG_GROUP_LIST`: Comma-separated list of allowed group/chat IDs
- `EMBEDDING_SERVICE_ADDRESS`: Custom address for embedding service
- `QDRANT_SERVICE_ADDRESS`: Custom address for Qdrant service

### Running with Docker Compose

1. Clone the repository:
   ```bash
   git clone https://github.com/korjavin/ragtgbot.git
   cd ragtgbot
   ```

2. Set your environment variables:
   ```bash
   export TELEGRAM_BOT_TOKEN=your_telegram_token
   export OPENAI_API_KEY=your_openai_api_key
   ```

3. Start all services using Docker Compose:
   ```bash
   docker-compose up -d
   ```

   Alternatively, use pre-built images from GitHub Container Registry:
   ```bash
   docker-compose -f docker-compose.ghcr.yml up -d
   ```

### Running Components Individually

#### Telegram Bot

```bash
cd cmd/tgbot
go run main.go
```

#### Embedding Service

```bash
cd embedding_service
pip install -r requirements.txt
python main.py
```

#### Backup Uploader

```bash
cd cmd/uploadbackup
go run main.go
```

## Configuration

The bot uses the following configuration options:

- **Embedding Service**: Address configurable via `EMBEDDING_SERVICE_ADDRESS` environment variable (default: "http://localhost:8000/embeddings")
- **Qdrant Service**: Address configurable via `QDRANT_SERVICE_ADDRESS` environment variable (default: "http://localhost:6333")
- **Collection Name**: Name of the collection in Qdrant (default: "chat_history")
- **OpenAI Model**: Model to use for generating responses (default: "gpt-4o-mini")
- **Vector Search Limit**: Number of similar messages to retrieve (default: 10)

## Security

The bot can be restricted to specific Telegram groups/chats:

- Set `TG_GROUP_LIST` environment variable with a comma-separated list of allowed group/chat IDs
- Example: `export TG_GROUP_LIST="-1001234567890,-1009876543210"`
- When restricted, the bot will:
  - Only store and process messages from allowed groups
  - Respond with an explanation message in other chats
  - Continue to ignore messages that don't mention it
- If `TG_GROUP_LIST` is not set, the bot will work in all chats

To find your group ID:
1. Add the bot to your group
2. Send a message in the group
3. Check the bot's logs for the chat ID number
4. Use this ID in the TG_GROUP_LIST

Example docker-compose configuration:
```yaml
services:
  tgbot:
    environment:
      - TG_GROUP_LIST=-1001234567890,-1009876543210
```

## Usage

1. Add the bot to a Telegram group or start a direct conversation with it.

2. Send regular messages to be stored in the vector database.

3. Mention the bot with a query to get an AI-generated response based on chat history:
   ```
   @your_bot_name what did we discuss yesterday about the project?
   ```

4. The bot will respond with:
   - An AI-generated answer based on the context
   - A list of the most relevant messages from the chat history

## Docker Images

The project provides Docker images for all components:

- **Telegram Bot**: `ghcr.io/korjavin/ragtgbot-tgbot:latest`
- **Embedding Service**: `ghcr.io/korjavin/ragtgbot-embedding-service:latest`

## Development

### Running tests

```bash
go test ./...
```

### Running linting

```bash
golangci-lint run
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License 