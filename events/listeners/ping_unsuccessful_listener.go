package listeners

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/DevloperAmanSingh/watcher/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type PingUnSuccessfulListener struct {
	ctx    context.Context
	logger *slog.Logger
	DB     *pgxpool.Pool
}

func (sl *PingUnSuccessfulListener) Handle(event core.Event) {
	e := event.(*events.PingUnSuccessful)
	fmt.Printf("%v is unhealthy, pushing to timescale DB and sending email out \n", e.Url)

	urlRepo := database.NewUrlRepository(sl.DB)
	url, err := urlRepo.FindById(sl.ctx, e.UrlId)
	if err != nil {
		sl.logger.Error("failed to find url", "error", err, "url_id", e.UrlId)
		return
	}

	//check if the previous status is healthy, if it is healthy, send email
	if url.Status == enums.Healthy {
		incidentRepo := database.NewIncidentRepository(sl.DB)
		if incidentErr := incidentRepo.Add(sl.ctx, url.Id); incidentErr != nil {
			sl.logger.Error("failed to open incident",
				"error", incidentErr, "url_id", url.Id, "url", url.Url)
		}

		if mailErr := core.SendEmail(core.SendEmailConfig{
			Recipients:  []string{url.ContactEmail},
			Subject:     "Your Site is DOWN",
			Content:     fmt.Sprintf("Your Site `%v` is DOWN. It went down at %v\n . Please check it out", url.Url, time.Now()),
			ContentType: "text/plain",
		}); mailErr != nil {
			sl.logger.Error("failed to send outage alert",
				"error", mailErr, "url_id", url.Id, "url", url.Url)
		}
	}

	urlRepository := database.NewUrlRepository(sl.DB)
	if err = urlRepository.UpdateStatus(sl.ctx, e.UrlId, enums.UnHealthy); err != nil {
		sl.logger.Error("failed to update url status",
			"error", err, "url_id", e.UrlId, "status", enums.UnHealthy.ToString())
		return
	}

	urlStatusRepo := database.NewUrlStatusRepository(sl.DB)
	if err = urlStatusRepo.Add(sl.ctx, e.UrlId, e.Healthy); err != nil {
		sl.logger.Error("failed to record check result",
			"error", err, "url_id", e.UrlId, "healthy", e.Healthy)
		return
	}
}

func NewPingUnSuccessfulListener(ctx context.Context, logger *slog.Logger, db *pgxpool.Pool) *PingUnSuccessfulListener {
	return &PingUnSuccessfulListener{
		logger: logger,
		ctx:    ctx,
		DB:     db,
	}
}
