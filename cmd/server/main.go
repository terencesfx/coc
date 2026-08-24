package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mushan/coc/internal/account"
	"github.com/mushan/coc/internal/campaign"
	"github.com/mushan/coc/internal/character"
	"github.com/mushan/coc/internal/config"
	"github.com/mushan/coc/internal/database"
	"github.com/mushan/coc/internal/dice"
	"github.com/mushan/coc/internal/httpapi"
	"github.com/mushan/coc/internal/maintenance"
	"github.com/mushan/coc/internal/notification"
	"github.com/mushan/coc/internal/rules/coc7"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dataLock, err := maintenance.AcquireDataLock(cfg.DatabasePath)
	if err != nil {
		logger.Error("data directory is already in use", "error", err)
		os.Exit(1)
	}
	defer dataLock.Close()
	db, err := database.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	accounts := account.NewStore(db)
	characters := character.NewStore(db)
	campaigns := campaign.NewStore(db, cfg.AssetDir)
	diceRolls := dice.NewStore(db)
	startedAt := time.Now().UTC()
	maintenanceService := maintenance.New(db, cfg.DatabasePath, cfg.BackupDir, startedAt, cfg.AssetDir, cfg.CustomOccupationsPath)
	notificationStore := notification.NewStore(db)
	notificationWorker := notification.NewWorker(db, logger)
	occupations, err := coc7.LoadOccupationCatalog(cfg.OfficialOccupationsPath, cfg.CustomOccupationsPath)
	if err != nil {
		logger.Error("occupation catalog initialization failed", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(logger, accounts, characters, campaigns, diceRolls, occupations, maintenanceService, notificationStore, cfg.CookieSecure),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go notificationWorker.Run(ctx)

	go func() {
		logger.Info("http server started", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("http server stopped")
}
