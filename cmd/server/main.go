package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo/internal/config"
	"todo/internal/database"
	"todo/internal/repository"
	"todo/internal/service"
	"todo/internal/web"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("load timezone: %v", err)
	}

	ctx := context.Background()
	dbpool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if cfg.AutoMigrate {
		if err := database.ApplyMigrations(ctx, dbpool, cfg.MigrationsDir); err != nil {
			log.Fatalf("apply migrations: %v", err)
		}
	}

	repo := repository.NewTaskRepository(dbpool)
	parser := service.NewTextParser(location)
	icsImporter := service.NewICSImporter(location, cfg.ICSImportHorizonDays)
	taskService := service.NewTaskService(repo, parser, icsImporter, location)

	handler, err := web.NewHandler(taskService, "web/templates", "web/static", cfg.MaxUploadSizeBytes, location)
	if err != nil {
		log.Fatalf("build handler: %v", err)
	}

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler.Router(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("todo server listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve http: %v", err)
		}
	}()

	waitForShutdown(server)
}

func waitForShutdown(server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
		return
	}

	fmt.Println("server stopped")
}
