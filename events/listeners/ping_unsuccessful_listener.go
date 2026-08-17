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
		sl.logger.Error("Error finding url: ", err, e)
		return
	}

	//check if the previous status is healthy, if it is healthy, send email
	if url.Status == enums.Healthy {
		incidentRepo := database.NewIncidentRepository(sl.DB)
		err := incidentRepo.Add(sl.ctx, url.Id)
		if err != nil {
			sl.logger.Error("Unable to log incident: ", err.Error(), url)
		}

		err = core.SendEmail(core.SendEmailConfig{
			Recipients:  []string{url.ContactEmail},
			Subject:     "Your Site is DOWN",
			Content:     fmt.Sprintf("Your Site `%v` is DOWN. It went down at %v\n . Please check it out", url.Url, time.Now()),
			ContentType: "text/plain",
		})
		if err != nil {
			sl.logger.Error("Error sending monitoring alert email: ", err, e)
		}
	}

	urlRepository := database.NewUrlRepository(sl.DB)
	err = urlRepository.UpdateStatus(sl.ctx, e.UrlId, enums.UnHealthy)
	if err != nil {
		sl.logger.Error(err.Error(), e)
		return
	}

	urlStatusRepo := database.NewUrlStatusRepository(sl.DB)
	err = urlStatusRepo.Add(sl.ctx, e.UrlId, e.Healthy)
	if err != nil {
		sl.logger.Error(err.Error(), e)
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
