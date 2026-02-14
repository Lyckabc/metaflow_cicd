package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/neunexus/metaflow_cicd/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openAPISpec is OpenAPI 3.0 spec for interactive docs (Swagger UI).
const openAPISpec = `{
  "openapi": "3.0.3",
  "info": { "title": "Metaflow CICD API", "version": "1.0.0" },
  "paths": {
    "/ci_projects": {
      "post": {
        "summary": "Create CI project",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["repo_url"],
                "properties": {
                  "service_name": { "type": "weknora" },
                  "repo_url": { "type": "https://github.com/Lyckabc/WeKnora" },
                  "branch": { "type": "dev", "default": "main" },
                  "registry_url": { "type": "https://registry.toji.homes/" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Created" },
          "400": { "description": "Bad request" }
        }
      }
    },
    "/ci_secrets": {
      "post": {
        "summary": "Create CI secret",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["key", "value", "description"],
                "properties": {
                  "key": { "type": "registry_password" },
                  "value": { "type": "secret123" },
                  "description": { "type": "Registry login password" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Created" },
          "400": { "description": "Bad request" }
        }
      }
    }
  }
}`

// swaggerUIHTML serves Swagger UI (like FastAPI /docs).
const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.presets.standalone
        ]
      });
    };
  </script>
</body>
</html>
`

func connectDB() *gorm.DB {
	host := os.Getenv("METAFLOW_CICD_DB_HOST")
	port := os.Getenv("METAFLOW_CICD_DB_PORT")
	user := os.Getenv("METAFLOW_CICD_DB_USER")
	password := os.Getenv("METAFLOW_CICD_DB_PASSWORD")
	dbname := os.Getenv("METAFLOW_CICD_DB_NAME")
	sslmode := os.Getenv("METAFLOW_CICD_DB_SSLMODE")
	if port == "" {
		port = "5432"
	}
	if sslmode == "" {
		sslmode = "disable"
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("DB connect failed:", err)
	}
	return db
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No .env file or failed to load (using process env):", err)
	}
	db := connectDB()
	repo, err := repository.New(db)
	if err != nil {
		log.Fatalln("Repository init (AutoMigrate):", err)
	}
	ctx := context.Background()

	http.HandleFunc("POST /ci_projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ServiceName string `json:"service_name"`
			RepoURL     string `json:"repo_url"`
			Branch      string `json:"branch"`
			RegistryURL string `json:"registry_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.RepoURL == "" {
			http.Error(w, "repo_url is required", http.StatusBadRequest)
			return
		}
		if body.Branch == "" {
			body.Branch = "main"
		}
		p := &repository.CIProject{
			ServiceName: body.ServiceName,
			RepoURL:     body.RepoURL,
			Branch:      body.Branch,
			RegistryURL: body.RegistryURL,
		}
		if err := repo.CreateProject(ctx, p); err != nil {
			http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": p.ID, "service_name": p.ServiceName})
	})

	http.HandleFunc("POST /ci_secrets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Key         string `json:"key"`
			Value       string `json:"value"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Key == "" || body.Value == "" || body.Description == "" {
			http.Error(w, "key, value, description are required", http.StatusBadRequest)
			return
		}
		s := &repository.CISecret{
			Key:         body.Key,
			Value:       body.Value,
			Description: body.Description,
		}
		if err := repo.CreateSecret(ctx, s); err != nil {
			http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID, "key": s.Key})
	})

	// OpenAPI spec (for Swagger UI)
	http.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(openAPISpec))
	})

	// Swagger UI (FastAPI /docs style)
	http.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(swaggerUIHTML))
	})

	host := os.Getenv("METAFLOW_CICD_API_HOST")
	port := os.Getenv("METAFLOW_CICD_API_PORT")
	if port == "" {
		port = "8059"
	}
	flag.StringVar(&host, "host", host, "listen host (e.g. 0.0.0.0 for LAN access)")
	flag.Parse()
	if host == "" {
		host = "0.0.0.0"
	}
	addr := host + ":" + port
	log.Printf("API server listening on %s (POST /ci_projects, POST /ci_secrets, GET /docs)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
