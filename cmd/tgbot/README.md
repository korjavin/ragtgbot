# Telegram RAG Bot with OpenAI Integration

This is a Retrieval-Augmented Generation (RAG) bot for Telegram that stores messages in a vector database, retrieves relevant messages when queried, and uses OpenAI's GPT-4-mini model to generate intelligent responses.

## Overview

The Telegram RAG Bot is designed to:

1. Store all messages it receives in a vector database (Qdrant) with their embeddings
2. Respond to queries by finding semantically similar messages in its database
3. Use OpenAI's GPT-4-mini model to generate intelligent responses based on retrieved messages
4. Provide both AI-generated answers and relevant messages from previous conversations

## How It Works

1. **Message Storage**: When the bot receives a regular message, it:
   - Generates embeddings for the message text using an embedding service
   - Stores the message text, username, and embedding in Qdrant (a vector database)

2. **Query Processing**: When the bot is mentioned with a query, it:
   - Extracts the query from the message
   - Generates embeddings for the query
   - Searches the vector database for the top 10 semantically similar messages
   - Constructs a prompt for OpenAI using these messages
   - Calls the OpenAI API to generate a response
   - Returns both the AI-generated answer and the top 5 most relevant messages

## Components

- **Telegram Bot**: Uses the telebot library to interact with the Telegram API
- **Embedding Service**: External service that generates vector embeddings for text
- **Qdrant Vector Database**: Stores messages and their embeddings for semantic search
- **OpenAI Integration**: Uses GPT-4-mini to generate intelligent responses based on context

## Requirements

- Go 1.23 or higher
- Telegram Bot Token (set as environment variable `TELEGRAM_BOT_TOKEN`)
- OpenAI API Key (set as environment variable `OPENAI_API_KEY`)
- Running embedding service at http://localhost:8000/embeddings
- Running Qdrant service with HTTP API at localhost:6333

## Configuration

The bot uses the following constants that can be modified in the code:

- `embeddingServiceAddress`: Address of the embedding service (default: "http://localhost:8000/embeddings")
- `qdrantServiceAddress`: Address of the Qdrant HTTP API (default: "http://localhost:6333")
- `collectionName`: Name of the collection in Qdrant (default: "chat_history")
- `openaiAPIURL`: OpenAI API URL (default: "https://api.openai.com/v1/chat/completions")
- `openaiModel`: OpenAI model to use (default: "gpt-4-mini")
- `vectorSearchLimit`: Number of similar messages to retrieve (default: 10)

## Usage

1. Set your Telegram Bot Token and OpenAI API Key as environment variables:
   ```
   export TELEGRAM_BOT_TOKEN=your_token_here
   export OPENAI_API_KEY=your_openai_api_key_here
   ```

2. Run the bot:
   ```
   go run cmd/tgbot/main.go
   ```

3. Using Docker:
   ```
   docker-compose up -d
   ```

4. Interact with the bot in Telegram:
   - Send regular messages to be stored
   - Mention the bot with a query to retrieve relevant messages and get an AI-generated response (e.g., "@your_bot_name what did we discuss yesterday?")

## Docker Support

The bot is available as a Docker image and can be run using Docker Compose. Two versions of the docker-compose file are available:

1. `docker-compose.yml`: Builds the images locally
2. `docker-compose.ghcr.yml`: Pulls pre-built images from GitHub Container Registry (ghcr.io)

## Logging

The bot includes comprehensive logging that shows:
- Connection status to services
- Message processing steps
- Query processing and search results
- OpenAI API interactions
- Any errors that occur during operation

This makes it easier to monitor the bot's operation and troubleshoot any issues.