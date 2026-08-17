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
	e := event.(*events.PingSuccessful)
	fmt.Printf("%v is healthy, pushing to timescale DB \n", e.Url)
	urlRepo := database.NewUrlRepository(sl.DB)
	url, err := urlRepo.FindById(sl.ctx, e.UrlId)
	if err != nil {
		sl.logger.Error("Error finding url: ", err, e)
		return
	}

	if url.Status == enums.UnHealthy {
		incidentRepo := database.NewIncidentRepository(sl.DB)
		err := incidentRepo.Resolve(sl.ctx, url.Id)
		if err != nil {
			sl.logger.Error("Unable to log incident as resolved: ", err.Error(), url)
		}

		err = core.SendEmail(core.SendEmailConfig{
			Recipients:  []string{url.ContactEmail},
			Subject:     "Your Site is now UP",
			Content:     fmt.Sprintf("Your Site `%v` is UP. It went up at %v. Good work", url.Url, time.Now()),
			ContentType: "text/plain",
		})
		if err != nil {
			sl.logger.Error("Error sending monitoring alert email: ", err, e)
		}
	}

	urlStatusRepo := database.NewUrlStatusRepository(sl.DB)
	err = urlStatusRepo.Add(sl.ctx, e.UrlId, e.Healthy)
	if err != nil {
		sl.logger.Error(err.Error(), e)
		return
	}

	urlRepository := database.NewUrlRepository(sl.DB)
	err = urlRepository.UpdateStatus(sl.ctx, e.UrlId, enums.Healthy)
	if err != nil {
		sl.logger.Error(err.Error(), e)
		return
	}
}

func NewPingSuccessfulListener(ctx context.Context, logger *slog.Logger, db *pgxpool.Pool) *PingSuccessfulListener {
	return &PingSuccessfulListener{
		logger: logger,
		ctx:    ctx,
		DB:     db,
	}
}
