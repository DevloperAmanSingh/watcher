package orchestrator

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/DevloperAmanSingh/watcher/events/listeners"
	"github.com/DevloperAmanSingh/watcher/logger"
	"github.com/DevloperAmanSingh/watcher/supervisor"
	"github.com/DevloperAmanSingh/watcher/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"sync"
	"time"
)

type Orchestrator struct {
	intervals     map[int]*worker.ParentWorker
	mutex         sync.RWMutex
	ctx           context.Context
	waitGroup     sync.WaitGroup
	RedisClient   *redis.Client
	Supervisor    *supervisor.Supervisor
	DB            *pgxpool.Pool
	UrlRepository database.UrlRepository
	Logger        *slog.Logger
	EventBus      *core.EventBus
}

func NewOrchestrator(ctx context.Context, rdC *redis.Client, pool *pgxpool.Pool) *Orchestrator {
	newLogger := logger.New()
	newEventBus := core.NewEventBus(newLogger)
	newEventBus.Subscribe("ping.successful", listeners.NewPingSuccessfulListener(ctx, newLogger, pool))
	newEventBus.Subscribe("ping.unsuccessful", listeners.NewPingUnSuccessfulListener(ctx, newLogger, pool))

	newSupervisor := supervisor.NewSupervisor(
		ctx,
		env.FetchInt("SUPERVISOR_POOL_FLUSH_BATCHSIZE", 100),
		time.Duration(env.FetchInt("SUPERVISOR_POOL_FLUSH_TIMEOUT", 5))*time.Second,
		newEventBus,
		pool,
	)

	return &Orchestrator{
		intervals:     make(map[int]*worker.ParentWorker),
		ctx:           ctx,
		RedisClient:   rdC,
		Supervisor:    newSupervisor,
		DB:            pool,
		UrlRepository: database.NewUrlRepository(pool),
		Logger:        newLogger,
		EventBus:      &newEventBus,
	}
}

func (o *Orchestrator) Start() {
	fmt.Println("Orchestrator is running")
	o.PrefillRedisList(o.ctx)
	for interval, parentWorker := range o.intervals {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		o.waitGroup.Add(1)
		go func() {
			for {
				select {
				case <-ticker.C:
					//fmt.Printf("tick for %v \n", interval)
					parentWorker.Signal <- true
				case <-o.ctx.Done():
					fmt.Println("Orchestrator is stopped")
					ticker.Stop()
					o.waitGroup.Done()
					return
				}
			}
		}()
	}
	o.waitGroup.Wait()
}

func (o *Orchestrator) AddInterval(interval enums.MonitoringFrequency, worker *worker.ParentWorker) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.intervals[interval.ToSeconds()] = worker
}

func (o *Orchestrator) AddIntervals(intervals []enums.MonitoringFrequency) {
	for _, interval := range intervals {
		workerGroup := worker.NewParentWorker(o.ctx, o.RedisClient, interval.ToSeconds(), o.Supervisor)
		workerGroup.Start()
		o.AddInterval(interval, workerGroup)
	}
}

func (o *Orchestrator) Intervals() []int {
	var intervals []int
	for interval, _ := range o.intervals {
		intervals = append(intervals, interval)
	}
	return intervals
}

func (o *Orchestrator) Stop() {}

func (o *Orchestrator) PrefillRedisList(ctx context.Context) {
	urls, err := database.NewUrlRepository(o.DB).FetchAll(ctx, 10, 0, database.UrlQueryFilter{})
	if err != nil {
		panic(err)
	}

	for _, interval := range o.Intervals() {
		o.RedisClient.Del(ctx, core.FormatRedisList(interval))
	}

	for _, url := range urls {
		o.RedisClient.LPush(ctx, core.FormatRedisList(url.MonitoringFrequency.ToSeconds()), url.Id)
		o.RedisClient.HSet(ctx, core.FormatRedisHash(url.MonitoringFrequency.ToSeconds()), url.Id, url)
	}
}
