package main

import (
	"context"
	"fmt"
	"log"

	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	qdrantHost     = "localhost:6334"
	collectionName = "Messages"
	vectorSize     = 384
)

func main() {
	// Connect to Qdrant
	conn, err := grpc.Dial(qdrantHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to Qdrant: %v", err)
	}
	defer conn.Close()

	collectionsClient := qdrant.NewCollectionsClient(conn)

	// Check if collection exists
	ctx := context.Background()
	_, err = collectionsClient.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: collectionName,
	})

	if err == nil {
		fmt.Printf("Collection %q already exists\n", collectionName)
		return
	}

	// Create new collection
	_, err = collectionsClient.Create(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     vectorSize,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})

	if err != nil {
		log.Fatalf("failed to create collection: %v", err)
	}

	fmt.Printf("Successfully created collection %q\n", collectionName)
}
