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
	"localoy-bot/internal/loco"
	"localoy-bot/internal/memory"
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

	// 3. Initialize domain services & memory
	systemService := service.NewSystemService()

	// Initialize Chat Memory (Redis or in-memory fallback)
	memoryStore := memory.NewStore(context.Background(), cfg.RedisURL, cfg.MaxChatHistory, log)
	defer memoryStore.Close()

	// Initialize Gemini AI Client, Upstream n8n Client, and Persona Service
	geminiClient := service.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	locoClient := loco.NewClient(
		cfg.LocoUpstreamURL,
		cfg.LocoWebhookSigningSecret,
		cfg.PartnerAPIURL,
		cfg.LocoServiceToken,
		log,
	)

	locoAIService := service.NewLocoAIService(
		locoClient,
		geminiClient,
		memoryStore,
		cfg.LocoChannelID,
		cfg.ChatResponseChance,
		log,
	)

	if locoAIService.IsConfigured() {
		log.Info("loco AI chat feature initialized",
			"channel_id", cfg.LocoChannelID,
			"model", cfg.GeminiModel,
			"upstream_enabled", locoClient.IsConfigured(),
			"max_history", cfg.MaxChatHistory,
		)
	} else {
		log.Info("loco AI chat feature is inactive (LOCO_CHANNEL_ID or AI keys not set)")
	}

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
		command.NewLocoCommand(locoClient, geminiClient),
		command.NewHelpCommand(cmdRegistry.All),
	)

	// 6. Instantiate Discord bot lifecycle manager
	discordBot, err := bot.New(cfg, log, cmdRegistry, systemService, locoAIService)
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
