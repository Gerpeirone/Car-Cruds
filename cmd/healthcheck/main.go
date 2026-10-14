package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+port+"/internal/ready", nil)
	if err != nil {
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("AUTH_SERVICE_TOKEN"))
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		os.Exit(1)
	}
}
