# Telegram RAG Bot

This is a Retrieval-Augmented Generation (RAG) bot for Telegram that stores messages in a vector database and retrieves relevant messages when queried.

## Overview

The Telegram RAG Bot is designed to:

1. Store all messages it receives in a vector database (Qdrant) with their embeddings
2. Respond to queries by finding semantically similar messages in its database
3. Provide relevant information based on previous conversations

## How It Works

1. **Message Storage**: When the bot receives a regular message, it:
   - Generates embeddings for the message text using an embedding service
   - Stores the message text and its embedding in Qdrant (a vector database)

2. **Query Processing**: When the bot is mentioned with a query, it:
   - Extracts the query from the message
   - Generates embeddings for the query
   - Searches the vector database for semantically similar messages
   - Returns the most relevant messages as a response

## Components

- **Telegram Bot**: Uses the telebot library to interact with the Telegram API
- **Embedding Service**: External service that generates vector embeddings for text
- **Qdrant Vector Database**: Stores messages and their embeddings for semantic search

## Requirements

- Go 1.23 or higher
- Telegram Bot Token (set as environment variable `TELEGRAM_BOT_TOKEN`)
- Running embedding service at http://localhost:8000/embeddings
- Running Qdrant service with HTTP API at localhost:6333

## Configuration

The bot uses the following constants that can be modified in the code:

- `embeddingServiceAddress`: Address of the embedding service (default: "http://localhost:8000/embeddings")
- `qdrantServiceAddress`: Address of the Qdrant HTTP API (default: "http://localhost:6333")
- `collectionName`: Name of the collection in Qdrant (default: "chat_history")

## Usage

1. Set your Telegram Bot Token as an environment variable:
   ```
   export TELEGRAM_BOT_TOKEN=your_token_here
   ```

2. Run the bot:
   ```
   go run cmd/tgbot/main.go
   ```

3. Interact with the bot in Telegram:
   - Send regular messages to be stored
   - Mention the bot with a query to retrieve relevant messages (e.g., "@your_bot_name what did we discuss yesterday?")

## Logging

The bot includes comprehensive logging that shows:
- Connection status to services
- Message processing steps
- Query processing and search results
- Any errors that occur during operation

This makes it easier to monitor the bot's operation and troubleshoot any issues.