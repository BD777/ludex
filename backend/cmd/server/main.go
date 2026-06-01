package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"local/ludex/internal/httpapi"
	"local/ludex/internal/storage"
)

func main() {
	addr := envAny([]string{"LUDEX_ADDR", "GMB_ADDR"}, "127.0.0.1:8787")
	dataDir := envAny([]string{"LUDEX_DATA_DIR", "GMB_DATA_DIR"}, ".data")

	store, err := storage.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := &http.Server{
		Addr:    addr,
		Handler: httpapi.New(store),
	}

	fmt.Printf("Ludex API listening on http://%s\n", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envAny(keys []string, fallback string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return fallback
}
