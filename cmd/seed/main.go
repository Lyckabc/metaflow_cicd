// Seed inserts metaflow_cicd project into DB. Run after migrations.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
	"github.com/neunexus/metaflow_cicd/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env (using process env)")
	}
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
		log.Fatalln("DB connect:", err)
	}
	repo, err := repository.New(db)
	if err != nil {
		log.Fatalln("Repository:", err)
	}
	ctx := context.Background()

	mainRepoURL := os.Getenv("METAFLOW_CICD_MAIN_REPO_URL")
	if mainRepoURL == "" {
		mainRepoURL = "https://github.com/Lyckabc/metaflow_cicd"
	}

	p := &repository.Project{
		ProjectName:    "metaflow_cicd",
		MainRepoURL:    mainRepoURL,
		TargetBranches: pq.StringArray{"main", "dev", "^feature/.*", "^release-.*"},
		CIConfigPath:   "flows/metaflow-ci.toml",
		CDConfigPath:   "flows/metaflow-ci.toml",
		IsActive:       true,
	}
	if err := repo.CreateProject(ctx, p); err != nil {
		log.Printf("CreateProject (may exist): %v", err)
	} else {
		log.Printf("Created project metaflow_cicd (id=%d)", p.ID)
	}
}
