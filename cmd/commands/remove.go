package commands

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"log/slog"
)

type RemoveCommand struct {
	*BaseCommand
}

func (mc *RemoveCommand) Arguments() []ArgumentContext {
	return []ArgumentContext{
		{
			Name:    "id",
			Usage:   "The ID of the URL to be removed.",
			Type:    enums.Int,
			Default: 0,
		},
	}
}

func (mc *RemoveCommand) Flags() []FlagContext {
	return []FlagContext{}
}

func (mc *RemoveCommand) Action(ctx context.Context, cmd CommandContext) error {
	id := cmd.Int("id")

	// Check if required argument is provided
	if id == 0 {
		return fmt.Errorf("ID is required")
	}

	pool := InitiateDB(ctx, mc.Log)
	urlRepository := database.NewUrlRepository(pool)

	url, err := urlRepository.FindById(ctx, id)
	if err != nil {
		fmt.Printf("Error removing url: %v", err)
		return err
	}

	err = urlRepository.Delete(ctx, url.Id)

	if err != nil {
		fmt.Printf("Error removing url: %v", err)
		return err
	}

	redisClient := InitiateRedis(ctx, mc.Log)
	err = RefreshRedisInterval(ctx, redisClient, pool, url.MonitoringFrequency)
	if err != nil {
		fmt.Printf("Error removing url from redis: %v", err)
		return err
	}

	fmt.Printf("URL successfully removing, ID: %v", id)
	return nil
}

func NewRemoveCommand(logger *slog.Logger) *RemoveCommand {
	return &RemoveCommand{
		BaseCommand: &BaseCommand{
			name:    "remove",
			aliases: []string{"rm"},
			usage:   "Remove a URL from the watcher monitoring process.",
			Log:     logger,
		},
	}
}
