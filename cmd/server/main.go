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
	"github.com/lib/pq"
	"github.com/neunexus/metaflow_cicd/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openAPISpec is OpenAPI 3.0 spec for interactive docs (Swagger UI).
const openAPISpec = `{
  "openapi": "3.0.3",
  "info": { "title": "Metaflow CICD API", "version": "2.0.0" },
  "paths": {
    "/sources": {
      "post": {
        "summary": "Create source",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "type"],
                "properties": {
                  "name": { "type": "string" },
                  "type": { "type": "string", "enum": ["git", "db", "api"] },
                  "host": { "type": "string" },
                  "access_token": { "type": "string" },
                  "description": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": { "201": { "description": "Created" }, "400": { "description": "Bad request" } }
      }
    },
    "/projects": {
      "post": {
        "summary": "Create project",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["project_name", "main_repo_url", "target_branches", "ci_config_path", "cd_config_path"],
                "properties": {
                  "project_name": { "type": "string" },
                  "main_repo_url": { "type": "string" },
                  "target_branches": { "type": "array", "items": { "type": "string" } },
                  "ci_source_name": { "type": "string" },
                  "ci_config_path": { "type": "string" },
                  "cd_source_name": { "type": "string" },
                  "cd_config_path": { "type": "string" },
                  "description": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": { "201": { "description": "Created" }, "400": { "description": "Bad request" } }
      }
    },
    "/secrets": {
      "post": {
        "summary": "Create secret",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["project_id", "secret_key", "secret_value"],
                "properties": {
                  "project_id": { "type": "integer" },
                  "secret_key": { "type": "string" },
                  "secret_value": { "type": "string" },
                  "scope": { "type": "string", "default": "prod" },
                  "description": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": { "201": { "description": "Created" }, "400": { "description": "Bad request" } }
      }
    }
  }
}`

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
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.presets.standalone]
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
		log.Fatalln("Repository init:", err)
	}
	ctx := context.Background()

	// POST /sources
	http.HandleFunc("POST /sources", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Name         string  `json:"name"`
			Type         string  `json:"type"`
			Host         *string `json:"host"`
			AccessToken  *string `json:"access_token"`
			WebhookSecret *string `json:"webhook_secret"`
			Description  *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Name == "" || body.Type == "" {
			http.Error(w, "name and type are required", http.StatusBadRequest)
			return
		}
		s := &repository.Source{
			Name:          body.Name,
			Type:          body.Type,
			Host:          body.Host,
			AccessToken:   body.AccessToken,
			WebhookSecret: body.WebhookSecret,
			Description:   body.Description,
		}
		if err := repo.CreateSource(ctx, s); err != nil {
			http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID, "name": s.Name})
	})

	// POST /projects
	http.HandleFunc("POST /projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ProjectName    string   `json:"project_name"`
			MainRepoURL    string   `json:"main_repo_url"`
			TargetBranches []string `json:"target_branches"`
			CISourceName   *string  `json:"ci_source_name"`
			CIConfigPath   string   `json:"ci_config_path"`
			CDSourceName   *string  `json:"cd_source_name"`
			CDConfigPath   string   `json:"cd_config_path"`
			Description    *string  `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.ProjectName == "" || body.MainRepoURL == "" || body.CIConfigPath == "" || body.CDConfigPath == "" {
			http.Error(w, "project_name, main_repo_url, ci_config_path, cd_config_path are required", http.StatusBadRequest)
			return
		}
		if len(body.TargetBranches) == 0 {
			body.TargetBranches = []string{"main"}
		}
		p := &repository.Project{
			ProjectName:    body.ProjectName,
			MainRepoURL:    body.MainRepoURL,
			TargetBranches: pq.StringArray(body.TargetBranches),
			CISourceName:   body.CISourceName,
			CIConfigPath:   body.CIConfigPath,
			CDSourceName:   body.CDSourceName,
			CDConfigPath:   body.CDConfigPath,
			Description:   body.Description,
		}
		if err := repo.CreateProject(ctx, p); err != nil {
			http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": p.ID, "project_name": p.ProjectName})
	})

	// POST /secrets
	http.HandleFunc("POST /secrets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ProjectID   int     `json:"project_id"`
			SecretKey   string  `json:"secret_key"`
			SecretValue string  `json:"secret_value"`
			Scope       string  `json:"scope"`
			Description *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.ProjectID == 0 || body.SecretKey == "" || body.SecretValue == "" {
			http.Error(w, "project_id, secret_key, secret_value are required", http.StatusBadRequest)
			return
		}
		if body.Scope == "" {
			body.Scope = "prod"
		}
		s := &repository.Secret{
			ProjectID:   body.ProjectID,
			SecretKey:   body.SecretKey,
			SecretValue: body.SecretValue,
			Scope:       body.Scope,
			Description: body.Description,
		}
		if err := repo.CreateSecret(ctx, s); err != nil {
			http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID, "secret_key": s.SecretKey})
	})

	http.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(openAPISpec))
	})
	http.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(swaggerUIHTML))
	})

	host := os.Getenv("METAFLOW_CICD_API_HOST")
	port := os.Getenv("METAFLOW_CICD_API_PORT")
	if port == "" {
		port = "8059"
	}
	flag.StringVar(&host, "host", host, "listen host")
	flag.Parse()
	if host == "" {
		host = "0.0.0.0"
	}
	addr := host + ":" + port
	log.Printf("API server listening on %s (POST /sources, POST /projects, POST /secrets, GET /docs)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
