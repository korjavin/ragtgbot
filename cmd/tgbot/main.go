package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	tele "gopkg.in/telebot.v3"
)

const (
	embeddingServiceAddress = "http://localhost:8000/embeddings" // Address of the embedding service
	qdrantServiceAddress    = "localhost:6333"                   // Address of the Qdrant service
	collectionName          = "chat_history"
)

type TextList struct {
	Texts []string `json:"texts"`
}

func getEmbeddings(texts []string) ([]float32, error) {
	jsonData, err := json.Marshal(TextList{Texts: texts})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(embeddingServiceAddress, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var embeddings []float32
	err = json.Unmarshal(body, &embeddings)
	if err != nil {
		return nil, err
	}

	return embeddings, nil
}

func main() {
	// Telegram Bot Token
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// Initialize Qdrant client
	ctx := context.Background()

	// Establish connection to gRPC server
	conn, err := grpc.Dial(qdrantServiceAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// Initialize the Qdrant clients
	collectionsClient := qdrant.NewCollectionsClient(conn)
	pointsClient := qdrant.NewPointsClient(conn)

	// Check if the collection exists
	_, err = collectionsClient.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: collectionName,
	})

	if err != nil {
		// Create collection if it doesn't exist
		_, err = collectionsClient.Create(ctx, &qdrant.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     384, // Embedding size
						Distance: qdrant.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("Failed to create collection: %v", err)
		}
		log.Println("Collection created successfully")
	} else {
		log.Println("Collection already exists")
	}

	// Telebot settings
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	// Create new bot
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Message handler
	b.Handle(tele.OnText, func(c tele.Context) error {
		// Check if the bot is mentioned
		if strings.Contains(c.Text(), "@"+b.Me.Username) {
			// Extract the query from the message
			query := strings.ReplaceAll(c.Text(), "@"+b.Me.Username, "")
			query = strings.TrimSpace(query)

			// Get embedding for the query
			queryEmbeddings, err := getEmbeddings([]string{query})
			if err != nil {
				log.Printf("Error getting embedding for query: %v", err)
				return c.Send("Error processing your query")
			}

			// Search the vector database
			searchResult, err := pointsClient.Search(ctx, &qdrant.SearchPoints{
				CollectionName: collectionName,
				Vector:         queryEmbeddings,
				Limit:          5,
			})
			if err != nil {
				log.Printf("Error searching vector database: %v", err)
				return c.Send("Error processing your query")
			}

			// Construct the answer from the search results
			var answer strings.Builder
			answer.WriteString("Here are some relevant messages:\n")
			for _, result := range searchResult.Result {
				payloadText := result.Payload["text"]
				answer.WriteString(fmt.Sprintf("- %s\n", payloadText.GetStringValue()))
			}

			// Send the answer
			return c.Send(answer.String())
		}

		// Calculate embedding for the message
		embeddings, err := getEmbeddings([]string{c.Text()})
		if err != nil {
			log.Printf("Error getting embedding: %v", err)
			return nil // Don't return an error to the user for background processing
		}

		// Store the message and its embedding in the vector database
		id := time.Now().UnixNano()
		pointId := &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Num{Num: uint64(id)},
		}

		payload := map[string]*qdrant.Value{
			"text": {
				Kind: &qdrant.Value_StringValue{StringValue: c.Text()},
			},
		}

		// Create a wait boolean pointer
		waitTrue := true

		points := []*qdrant.PointStruct{{
			Id:      pointId,
			Vectors: qdrant.NewVectors(embeddings...),
			Payload: payload,
		}}

		_, err = pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: collectionName,
			Wait:           &waitTrue,
			Points:         points,
		})

		if err != nil {
			log.Printf("Error adding to vector database: %v", err)
			return nil
		}

		return nil
	})

	// Start the bot
	go func() {
		b.Start()
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Shutdown the bot
	b.Stop()
	log.Println("Telegram bot stopped")
}
