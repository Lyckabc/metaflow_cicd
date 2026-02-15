package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/neunexus/metaflow_cicd/internal/handler"
	"go.temporal.io/sdk/client"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal:7233"
	}
	if strings.HasPrefix(hostPort, "localhost:") {
		hostPort = "127.0.0.1" + hostPort[len("localhost"):]
	}
	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalln("Temporal client:", err)
	}
	defer c.Close()

	tr := handler.NewTrigger(c)
	// Convoy endpoint secret (same as setup_convoy_github.py --github-secret) for X-Convoy-Signature verification
	secret := os.Getenv("CONVOY_ENDPOINT_SECRET")
	if secret == "" {
		secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	}
	if secret == "" {
		log.Print("Warning: CONVOY_ENDPOINT_SECRET not set; webhook signature verification disabled")
	}
	webhook := handler.NewWebhookAdapter(tr, secret)

	http.Handle("POST /trigger", tr)
	http.Handle("POST /webhooks/github", webhook)
	http.Handle("GET /webhooks/github", webhook)
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := os.Getenv("TRIGGER_LISTEN")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}
	log.Printf("Trigger listening on %s (POST /trigger, POST /webhooks/github)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
