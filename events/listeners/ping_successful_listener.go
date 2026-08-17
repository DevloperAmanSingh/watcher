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

type PingSuccessfulListener struct {
	ctx    context.Context
	logger *slog.Logger
	DB     *pgxpool.Pool
}

func (sl *PingSuccessfulListener) Handle(event core.Event) {
	e, ok := event.(*events.PingSuccessful)
	if !ok {
		sl.logger.Error("unexpected event type on ping.successful", "event", event.Name())
		return
	}
	fmt.Printf("%v is healthy, pushing to timescale DB \n", e.Url)
	urlRepo := database.NewUrlRepository(sl.DB)
	url, err := urlRepo.FindById(sl.ctx, e.UrlId)
	if err != nil {
		sl.logger.Error("failed to find url", "error", err, "url_id", e.UrlId)
		return
	}

	urlStatusRepo := database.NewUrlStatusRepository(sl.DB)
	if statusErr := urlStatusRepo.Add(sl.ctx, e.UrlId, e.Healthy); statusErr != nil {
		sl.logger.Error("failed to record check result",
			"error", statusErr, "url_id", e.UrlId, "healthy", e.Healthy)
	}

	// A success ends any run of failures, whether or not it was long enough to
	// have opened an incident. Without this the counter would accumulate across
	// unrelated blips and eventually cross the threshold on its own.
	if _, resetErr := urlRepo.ResetFailures(sl.ctx, url.Id); resetErr != nil {
		sl.logger.Error("failed to reset consecutive failures",
			"error", resetErr, "url_id", url.Id, "url", url.Url)
	}

	if statusErr := urlRepo.UpdateStatus(sl.ctx, e.UrlId, enums.Healthy); statusErr != nil {
		sl.logger.Error("failed to update url status",
			"error", statusErr, "url_id", e.UrlId, "status", enums.Healthy.ToString())
	}

	// As on the outage path, the write decides the transition rather than the
	// status read: whichever concurrent result actually closes the open
	// incident owns the recovery notification, and the rest stay silent.
	incidentRepo := database.NewIncidentRepository(sl.DB)
	resolved, err := incidentRepo.Resolve(sl.ctx, url.Id)
	if err != nil {
		sl.logger.Error("failed to resolve incident",
			"error", err, "url_id", url.Id, "url", url.Url)
	}

	if resolved {
		sl.logger.Info("site recovered", "url_id", url.Id, "url", url.Url)

		if mailErr := core.SendEmail(core.SendEmailConfig{
			Recipients:  []string{url.ContactEmail},
			Subject:     "Your Site is now UP",
			Content:     fmt.Sprintf("Your Site `%v` is UP. It went up at %v. Good work", url.Url, time.Now()),
			ContentType: "text/plain",
		}); mailErr != nil {
			sl.logger.Error("failed to send recovery alert",
				"error", mailErr, "url_id", url.Id, "url", url.Url)
		}
	}
}

func NewPingSuccessfulListener(ctx context.Context, logger *slog.Logger, db *pgxpool.Pool) *PingSuccessfulListener {
	return &PingSuccessfulListener{
		logger: logger,
		ctx:    ctx,
		DB:     db,
	}
}
