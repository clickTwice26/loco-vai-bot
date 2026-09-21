# 🤖 Localoy Discord Bot

A production-ready Discord Bot built with **Go** and the **[discordgo](https://github.com/bwmarrin/discordgo)** framework, following idiomatic Go conventions, clean architecture, and modular design.

---

## 🏛️ Project Architecture

```
localoy-bot/
├── cmd/
│   └── bot/
│       └── main.go               # Application entry point & graceful lifecycle orchestration
├── config/
│   ├── config.go                 # Strongly-typed environment configuration & validation
│   └── config_test.go            # Unit tests for configuration
├── internal/
│   ├── bot/
│   │   ├── bot.go                # Bot lifecycle manager (session setup, intents, connect, disconnect)
│   │   └── middleware.go         # Command middleware pipeline (panic recovery, latency logging)
│   ├── command/
│   │   ├── command.go            # Command interface, registry, routing, & Discord sync
│   │   ├── command_test.go       # Unit tests for command registry
│   │   ├── ping.go               # /ping - Latency, heartbeat, uptime & runtime memory stats
│   │   ├── echo.go               # /echo - Option parsing & ephemeral message demo
│   │   ├── userinfo.go           # /userinfo - User target inspection & Discord embeds
│   │   └── help.go               # /help - Auto-generated interactive command directory
│   ├── event/
│   │   ├── ready.go              # Ready event handler (presence & initial logging)
│   │   ├── guild.go              # Guild join/leave events & dynamic presence updates
│   │   └── message.go            # MessageCreate event handler (e.g. mention nudges)
│   └── service/
│       ├── system.go             # Domain business logic (runtime metrics, uptime)
│       └── system_test.go        # Unit tests for domain services
├── pkg/
│   └── logger/
│       └── logger.go             # High-performance structured logging with slog (JSON & Text)
├── .env.example                  # Environment configuration template
├── .gitignore                    # Git ignore file
├── Dockerfile                    # Multi-stage lightweight production Docker build
├── docker-compose.yml            # Docker Compose setup
├── Makefile                      # Build, test, lint, and run scripts
├── go.mod                        # Go module dependencies
└── go.sum                        # Checksums for dependencies
```

---

## 📐 Architecture Highlights & Design Principles

```mermaid
flowchart TD
    subgraph Discord Gateway
        GW[Discord API & Gateway]
    end

    subgraph Bot Layer
        Main[cmd/bot/main.go] --> Config[config]
        Main --> Logger[pkg/logger]
        Main --> Bot[internal/bot]
        Bot --> Registry[internal/command Registry]
        Bot --> Events[internal/event Handlers]
    end

    subgraph Command Pipeline
        Registry --> MW1[Recovery Middleware]
        MW1 --> MW2[Logging Middleware]
        MW2 --> Router{Slash Command Router}
        Router --> PingCmd[/ping Command]
        Router --> EchoCmd[/echo Command]
        Router --> UserCmd[/userinfo Command]
        Router --> HelpCmd[/help Command]
    end

    subgraph Domain & Services
        PingCmd --> SysService[internal/service SystemService]
    end

    GW <-->|WebSocket / REST| Bot
```

1. **Clean Separation of Concerns**:
   - `cmd/bot`: Pure orchestration and graceful shutdown handling.
   - `internal/bot`: Discord session lifecycle, intent flags, and connection states.
   - `internal/command`: Strongly-typed command interface, dynamic registration, middleware chaining, and automated Discord API synchronization.
   - `internal/event`: Isolated gateway event listeners (Ready, Guilds, Messages).
   - `internal/service`: Core domain business logic decoupled from Discord transport.
   - `pkg/`: Reusable packages (structured `slog` logging).

2. **Middleware Pipeline**:
   - `RecoveryMiddleware`: Intercepts and recovers from unexpected handler panics, logging stack traces and alerting the user gracefully without crashing the bot.
   - `LoggingMiddleware`: Traces every command invocation with user ID, guild ID, channel ID, and execution duration in milliseconds.

3. **Fast Development & Production Sync**:
   - Set `GUILD_ID` in `.env` for instant slash command updates in your test server during development.
   - Leave `GUILD_ID` empty in production to sync commands globally across all Discord servers.

4. **Structured Logging (`log/slog`)**:
   - Supports human-readable `text` format for local development and structured `json` format for production container logging.

5. **Graceful Shutdown**:
   - Listens for `SIGINT` / `SIGTERM` signals and cleanly closes gateway sessions and active connections with a configurable timeout.

---

## 🚀 Quick Start

### 1. Prerequisites

- [Go](https://golang.org/dl/) (1.22+ recommended)
- A Discord Application & Bot Token from the [Discord Developer Portal](https://discord.com/developers/applications)

### 2. Configure Environment

Copy the `.env.example` template:

```bash
cp .env.example .env
```

Edit `.env` and configure your credentials:

```env
DISCORD_TOKEN=your_bot_token_here
APP_ID=your_discord_application_id
GUILD_ID=your_development_server_id # Optional: for instant dev syncing
LOG_LEVEL=DEBUG
LOG_FORMAT=text
```

### 3. Run the Bot

Using Make:

```bash
make run
```

Or using Go directly:

```bash
go run ./cmd/bot
```

---

## 🧪 Testing & Validation

Run all unit tests:

```bash
make test
```

Build the release binary:

```bash
make build
```

---

## 🐳 Running with Docker

Build and run using Docker Compose:

```bash
docker compose up -d --build
```

View live logs:

```bash
docker compose logs -f
```

---

## 🛠️ How to Add a New Slash Command

1. Create a new file under `internal/command/`, e.g. `internal/command/roll.go`:

```go
package command

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/bwmarrin/discordgo"
)

type RollCommand struct{}

func NewRollCommand() *RollCommand {
	return &RollCommand{}
}

func (c *RollCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "roll",
		Description: "Roll a random number between 1 and 100",
	}
}

func (c *RollCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	result := rand.Intn(100) + 1
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎲 You rolled: **%d**!", result),
		},
	})
}
```

2. Register your command in `cmd/bot/main.go`:

```go
cmdRegistry.Register(
    command.NewPingCommand(systemService),
    command.NewEchoCommand(),
    command.NewUserInfoCommand(),
    command.NewRollCommand(), // <-- Register here
    command.NewHelpCommand(cmdRegistry.All),
)
```

Your command will automatically sync with Discord on next startup and will be listed in `/help`!

---

## 📡 How to Add a New Event Listener

1. Create a handler function in `internal/event/`, e.g. `internal/event/reaction.go`:

```go
package event

import (
	"log/slog"
	"github.com/bwmarrin/discordgo"
)

func OnMessageReactionAdd(logger *slog.Logger) func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	return func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		logger.Debug("reaction added", "emoji", r.Emoji.Name, "user_id", r.UserID)
	}
}
```

2. Register it in `internal/bot/bot.go` inside `Start()`:

```go
b.session.AddHandler(event.OnMessageReactionAdd(b.logger))
```

---

## 📄 License

MIT License. Feel free to use and adapt this project structure for any Discord bot.
