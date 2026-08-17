package supervisor

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
	"time"
)

//This takes output from child workers as input
//Processes it for analytics with Timescale DB
//Dispatches Event for when a URL is unreachable

type Supervisor struct {
	WorkPool  chan Task
	BatchSize int
	Timeout   time.Duration
	ctx       context.Context
	WaitGroup *sync.WaitGroup
	EventBus  core.EventBus
	DB        *pgxpool.Pool
}

type Task struct {
	Healthy bool
	Url     string
	UrlId   int
}

func (s *Supervisor) Activate() {
	buffer := make([]Task, 0, s.BatchSize)
	ticker := time.NewTicker(s.Timeout)

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case task := <-s.WorkPool:
				buffer = append(buffer, task)
				if len(buffer) >= s.BatchSize {
					s.flush(buffer)
					buffer = buffer[:0]
				}
			case <-ticker.C:
				if len(buffer) > 0 {
					s.flush(buffer)
					buffer = buffer[:0]
				}
			}
		}
	}()
}

func (s *Supervisor) flush(buffer []Task) {
	for _, task := range buffer {
		fmt.Printf("supervisor picked up new task %v\n", task.Url)
		if task.Healthy {
			s.EventBus.Dispatch(&events.PingSuccessful{
				UrlId:   task.UrlId,
				Healthy: task.Healthy,
				Url:     task.Url,
			})
		} else {
			s.EventBus.Dispatch(&events.PingUnSuccessful{
				UrlId:   task.UrlId,
				Healthy: task.Healthy,
				Url:     task.Url,
			})
		}
	}
}

func NewSupervisor(ctx context.Context, batchSize int, Timeout time.Duration, eventBus core.EventBus, db *pgxpool.Pool) *Supervisor {
	return &Supervisor{
		WorkPool:  make(chan Task, batchSize),
		ctx:       ctx,
		BatchSize: batchSize,
		Timeout:   Timeout,
		WaitGroup: &sync.WaitGroup{},
		EventBus:  eventBus,
		DB:        db,
	}
}
