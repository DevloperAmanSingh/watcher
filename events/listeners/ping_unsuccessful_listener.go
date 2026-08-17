package listeners

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/DevloperAmanSingh/watcher/env"
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
	e, ok := event.(*events.PingUnSuccessful)
	if !ok {
		sl.logger.Error("unexpected event type on ping.unsuccessful", "event", event.Name())
		return
	}
	fmt.Printf("%v is unhealthy, pushing to timescale DB and sending email out \n", e.Url)

	urlRepo := database.NewUrlRepository(sl.DB)
	url, err := urlRepo.FindById(sl.ctx, e.UrlId)
	if err != nil {
		sl.logger.Error("failed to find url", "error", err, "url_id", e.UrlId)
		return
	}

	// Record the observation first. A check that does not cross the alert
	// threshold is still a data point, and uptime reporting is computed from
	// this series rather than from incidents.
	urlStatusRepo := database.NewUrlStatusRepository(sl.DB)
	if statusErr := urlStatusRepo.Add(sl.ctx, e.UrlId, e.Healthy); statusErr != nil {
		sl.logger.Error("failed to record check result",
			"error", statusErr, "url_id", e.UrlId, "healthy", e.Healthy)
	}

	// One failed check is not an outage. The counter is incremented and read in
	// a single statement, so concurrent results for this URL are serialized by
	// the row lock and each caller sees a distinct value.
	failures, err := urlRepo.RecordFailure(sl.ctx, url.Id)
	if err != nil {
		sl.logger.Error("failed to record consecutive failure",
			"error", err, "url_id", url.Id, "url", url.Url)
		return
	}

	threshold := failureThreshold()
	if failures < threshold {
		sl.logger.Debug("failure below alert threshold",
			"url_id", url.Id, "url", url.Url, "failures", failures, "threshold", threshold)
		return
	}

	if statusErr := urlRepo.UpdateStatus(sl.ctx, e.UrlId, enums.UnHealthy); statusErr != nil {
		sl.logger.Error("failed to update url status",
			"error", statusErr, "url_id", e.UrlId, "status", enums.UnHealthy.ToString())
	}

	// Opening the incident is what decides whether this result is the
	// transition, not the status read above. Between that read and here, a
	// concurrent result for the same URL may already have opened the incident;
	// only the caller whose insert actually created a row owns the alert.
	//
	// This is the whole of the exactly-once guarantee: the pipeline may
	// deliver a result more than once, but the notification is emitted by
	// whichever delivery won the insert, and by no other.
	incidentRepo := database.NewIncidentRepository(sl.DB)
	opened, err := incidentRepo.Open(sl.ctx, url.Id)
	if err != nil {
		sl.logger.Error("failed to open incident",
			"error", err, "url_id", url.Id, "url", url.Url)
	}

	if opened {
		sl.logger.Info("site went down", "url_id", url.Id, "url", url.Url)

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
}

// failureThreshold is how many consecutive failed checks must be observed
// before a URL is declared down. One is the previous behavior: alert on the
// first failure.
func failureThreshold() int {
	threshold := env.FetchInt("FAILURE_THRESHOLD", 3)
	if threshold < 1 {
		return 1
	}
	return threshold
}

func NewPingUnSuccessfulListener(ctx context.Context, logger *slog.Logger, db *pgxpool.Pool) *PingUnSuccessfulListener {
	return &PingUnSuccessfulListener{
		logger: logger,
		ctx:    ctx,
		DB:     db,
	}
}
