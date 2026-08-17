package commands

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"log/slog"
)

type AddCommand struct {
	*BaseCommand
}

func (mc *AddCommand) Arguments() []ArgumentContext {
	return []ArgumentContext{
		{
			Name:    "url",
			Usage:   "The URL of the watcher monitoring process.",
			Type:    enums.String,
			Default: "",
		},
		{
			Name:    "http_method",
			Usage:   "The HTTP method that URL will be called with. Options are: get,post,patch,put,delete",
			Type:    enums.String,
			Default: "get",
		},
		{
			Name:    "frequency",
			Usage:   "The Frequency the URL will be called. Options are: ten_seconds,thirty_seconds,five_minutes,thirty_minutes,one_hour,twelve_hours,twenty_four_hours",
			Type:    enums.String,
			Default: "five_minutes",
		},
		{
			Name:    "contact_email",
			Usage:   "The email an alert will be sent to if the URL is unreachable.",
			Type:    enums.String,
			Default: "",
		},
	}
}

func (mc *AddCommand) Flags() []FlagContext {
	return []FlagContext{}
}

func (mc *AddCommand) Action(ctx context.Context, cmd CommandContext) error {
	url := cmd.String("url")
	httpMethod := cmd.String("http_method")
	frequency := cmd.String("frequency")
	contactEmail := cmd.String("contact_email")

	// Check if required argument is provided
	if url == "" {
		return fmt.Errorf("url is required")
	}

	if contactEmail == "" {
		return fmt.Errorf("contact_email is required")
	}

	if httpMethod == "" {
		httpMethod = enums.Get.ToString()
	}

	if frequency == "" {
		frequency = enums.FiveMinutes.ToString()
	}

	pool := InitiateDB(ctx, mc.Log)
	urlRepository := database.NewUrlRepository(pool)

	parsedHttpMethod, err := enums.ParseHttpMethod(httpMethod)
	if err != nil {
		fmt.Printf("Error parsing http method: %v", err)
		return err
	}

	parsedFrequency, err := enums.ParseMonitoringFrequency(frequency)
	if err != nil {
		fmt.Printf("Error parsing frequency: %v", err)
		return err
	}

	id, err := urlRepository.Add(
		ctx,
		url,
		parsedHttpMethod,
		parsedFrequency,
		contactEmail,
	)

	if err != nil {
		mc.Log.Error("failed to add url", "error", err, "url", url)
		fmt.Printf("Error adding url")
		return err
	}

	redisClient := InitiateRedis(ctx, mc.Log)
	err = RefreshRedisInterval(ctx, redisClient, pool, parsedFrequency)
	if err != nil {
		mc.Log.Error("failed to sync url to redis",
			"error", err,
			"url_id", id,
			"frequency", parsedFrequency.ToString())
		fmt.Printf("Error adding url to redis")
		return err
	}

	fmt.Printf("URL successfully added, ID: %v", id)
	return nil
}

func NewAddCommand(logger *slog.Logger) *AddCommand {
	return &AddCommand{
		BaseCommand: &BaseCommand{
			name:    "add",
			aliases: []string{"a"},
			usage:   "Add a URL to the watcher monitoring process.",
			Log:     logger,
		},
	}
}
