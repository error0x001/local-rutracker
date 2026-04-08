package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/error0x001/rutracker/internal/config"
	"github.com/error0x001/rutracker/internal/db"
	"github.com/error0x001/rutracker/internal/migrator"
)

func main() {
	cfg := config.Load()

	filePath := flag.String("file", cfg.Migrator.FilePath, "Path to .xml.xz archive")
	runMigrations := flag.Bool("migrate", true, "Run DB migrations before import")
	flag.Parse()

	if *filePath == "" {
		fmt.Println("Usage: migrator -file=path/to/rutracker.xml.xz")
		fmt.Println()
		fmt.Println("Resumes automatically from last saved checkpoint in DB.")
		fmt.Println("Environment: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE")
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DB.DSN())
	if err != nil {
		log.Fatalf("DB connect error: %v", err)
	}
	defer pool.Close()

	fmt.Println("Connected to database:", cfg.DB.Host)

	if *runMigrations {
		fmt.Println("Running migrations...")
		if err := db.RunMigrations(ctx, pool); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}

	m := migrator.NewMigrator(pool, cfg.Migrator.ProgressEvery)
	if err := m.MigrateFile(ctx, *filePath); err != nil {
		log.Fatalf("Migration error: %v", err)
	}
}
