package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"localoy-bot/config"
	"localoy-bot/internal/bot"
	"localoy-bot/internal/command"
	"localoy-bot/internal/server"
	"localoy-bot/internal/service"
	"localoy-bot/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 2. Initialize structured logger
	log := logger.Setup(cfg.LogLevel, cfg.LogFormat)
	log.Info("starting localoy-bot",
		"env", cfg.Environment,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
	)

	// 3. Initialize domain services
	systemService := service.NewSystemService()

	// 4. Initialize command registry and attach middlewares
	cmdRegistry := command.NewRegistry(log)
	cmdRegistry.Use(
		bot.RecoveryMiddleware(log),
		bot.LoggingMiddleware(log),
	)

	// 5. Register application slash commands
	cmdRegistry.Register(
		command.NewPingCommand(systemService),
		command.NewEchoCommand(),
		command.NewUserInfoCommand(),
		command.NewHelpCommand(cmdRegistry.All),
	)

	// 6. Instantiate Discord bot lifecycle manager
	discordBot, err := bot.New(cfg, log, cmdRegistry, systemService)
	if err != nil {
		return fmt.Errorf("failed to initialize bot: %w", err)
	}

	// 7. Start lightweight HTTP health server (for platforms like Cubicle / Docker)
	healthServer := server.New(cfg.Port, log)
	healthServer.Start()

	// 8. Setup graceful shutdown via OS signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// 9. Start the bot
	if err := discordBot.Start(ctx); err != nil {
		_ = healthServer.Shutdown(context.Background())
		return fmt.Errorf("failed to start bot: %w", err)
	}

	log.Info("bot is operational. Press CTRL+C to terminate.")

	// 10. Block until interrupt signal received
	<-ctx.Done()
	log.Info("shutdown signal received, commencing graceful teardown...")

	// 11. Perform graceful shutdown with a timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Warn("HTTP health server shutdown error", "error", err)
	}

	if err := discordBot.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("error during graceful shutdown: %w", err)
	}

	log.Info("bot stopped gracefully. Goodbye!")
	return nil
}
