package commands

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"log/slog"
	"strings"
)

type ListCommand struct {
	*BaseCommand
}

func (mc *ListCommand) Flags() []FlagContext {
	return []FlagContext{
		{
			Name:    "page",
			Usage:   "Set the page of the results list",
			Default: 1,
			Type:    enums.Int,
		},
		{
			Name:    "per_page",
			Usage:   "Set the number of results per page",
			Default: 20,
			Type:    enums.Int,
		},
		{
			Name:    "http_method",
			Usage:   "Filter results by http method",
			Default: "",
			Type:    enums.String,
		},
		{
			Name:    "frequency",
			Usage:   "Filter results by frequency",
			Default: "",
			Type:    enums.String,
		},
		{
			Name:    "status",
			Usage:   "Filter results by status",
			Default: "",
			Type:    enums.String,
		},
	}
}

func (mc *ListCommand) Action(ctx context.Context, cmd CommandContext) error {
	page := cmd.IntFlag("page")
	perPage := cmd.IntFlag("per_page")
	httpMethod := cmd.StringFlag("http_method")
	frequency := cmd.StringFlag("frequency")
	status := cmd.StringFlag("status")

	if page < 1 {
		return fmt.Errorf("page must be greater than 0")
	}
	if perPage < 1 {
		return fmt.Errorf("per_page must be greater than 0")
	}

	offset := (page - 1) * perPage

	filter := database.UrlQueryFilter{}
	if httpMethod != "" {
		parsedHttpMethod, err := enums.ParseHttpMethod(httpMethod)
		if err != nil {
			fmt.Printf("failed to fetch URLs: %v", err)
			return err
		}
		filter.HttpMethod = parsedHttpMethod
	}

	if frequency != "" {
		parsedFrequency, err := enums.ParseMonitoringFrequency(frequency)
		if err != nil {
			fmt.Printf("failed to fetch URLs: %v", err)
			return err
		}
		filter.Frequency = parsedFrequency
	}

	if status != "" {
		parsedStatus, err := enums.ParseSiteHealth(status)
		if err != nil {
			fmt.Printf("failed to fetch URLs: %v", err)
			return err
		}
		filter.Status = parsedStatus
	}

	pool := InitiateDB(ctx, mc.Log)
	urlRepository := database.NewUrlRepository(pool)

	urls, err := urlRepository.FetchAll(ctx, perPage+1, offset, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch URLs: %w", err)
	}

	if len(urls) == 0 {
		fmt.Println("No URLs found")
		return nil
	}

	hasMore := len(urls) > perPage
	if hasMore {
		urls = urls[:perPage]
	}

	hasPrevious := page > 1

	DisplayUrls(urls, page, offset, hasPrevious, hasMore)

	return nil
}

func NewListCommand(logger *slog.Logger) *ListCommand {
	return &ListCommand{
		BaseCommand: &BaseCommand{
			name:    "list",
			aliases: []string{"ls"},
			usage:   "List the URLs the watcher is guarding.",
			Log:     logger,
		},
	}
}

func DisplayUrls(urls []database.Url, page int, offset int, hasPrevious bool, hasMore bool) {
	fmt.Printf("Page %d (showing %d results)\n", page, len(urls))
	if hasPrevious {
		fmt.Printf("← Previous: --page=%d | ", page-1)
	}
	if hasMore {
		fmt.Printf("→ Next: --page=%d", page+1)
	}
	fmt.Println("\n" + strings.Repeat("-", 60))

	for i, url := range urls {
		fmt.Printf("%d. %s\n", offset+i+1, url.Url)
		fmt.Printf("   ID: %v | Method: %s | Status: %s | Frequency: %s\n",
			url.Id,
			url.HttpMethod.ToString(),
			url.Status.ToString(),
			url.MonitoringFrequency.ToString())
		fmt.Printf("   Contact: %s\n", url.ContactEmail)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("-", 60))
	if hasMore {
		fmt.Printf("More results available. Use --page=%d to continue.\n", page+1)
	} else {
		fmt.Println("✓ End of results")
	}
}
