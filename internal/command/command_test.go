package command

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type dummyCommand struct {
	name string
}

func (d *dummyCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        d.name,
		Description: "Dummy description for " + d.name,
	}
}

func (d *dummyCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return nil
}

func TestCommandRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := NewRegistry(logger)

	cmd1 := &dummyCommand{name: "test1"}
	cmd2 := &dummyCommand{name: "test2"}

	reg.Register(cmd1, cmd2)

	if len(reg.All()) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(reg.All()))
	}

	found, ok := reg.Get("test1")
	if !ok || found.Definition().Name != "test1" {
		t.Errorf("failed to retrieve registered command test1")
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Errorf("expected nonexistent command to not be found")
	}

	defs := reg.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if defs[0].Name != "test1" || defs[1].Name != "test2" {
		t.Errorf("unexpected definitions order or names")
	}
}
