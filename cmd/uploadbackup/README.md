# Telegram Backup Uploader

This tool parses a Telegram group backup in JSON format, extracts the text from each message, calculates embeddings using an embedding service, and saves the message text, username, and embeddings to a Qdrant vector database.

## Usage

1.  Make sure you have a Telegram group backup in JSON format (e.g., `testdata/result.json`).
2.  Start the Qdrant database and the embedding service using `docker-compose up`.
3.  Run the `uploadbackup` tool: `go run cmd/uploadbackup/main.go`.

The tool will read the JSON file, group messages by time/size, and save the data to the Qdrant database in the `chat_history` collection.