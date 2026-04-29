package main

import (
	"log"
	"seo-crawler/internal/server"
)

func main() {
	srv := server.New()
	log.Println("SEO Crawler starting on :8080")
	log.Fatal(srv.Start(":8080"))
}
