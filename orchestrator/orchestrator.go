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

func (o *Orchestrator) Start() error {
	fmt.Println("Orchestrator is running")
	if err := o.PrefillRedisList(o.ctx); err != nil {
		return fmt.Errorf("prefilling work set: %w", err)
	}
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
	return nil
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

func (o *Orchestrator) PrefillRedisList(ctx context.Context) error {
	for _, interval := range o.Intervals() {
		if err := o.RedisClient.Del(ctx, core.FormatRedisList(interval)).Err(); err != nil {
			return fmt.Errorf("clearing work list for interval %d: %w", interval, err)
		}
	}

	loaded := 0
	err := o.UrlRepository.Each(ctx, database.UrlQueryFilter{}, func(url database.Url) error {
		seconds := url.MonitoringFrequency.ToSeconds()
		if err := o.RedisClient.LPush(ctx, core.FormatRedisList(seconds), url.Id).Err(); err != nil {
			return fmt.Errorf("queueing url %d: %w", url.Id, err)
		}
		if err := o.RedisClient.HSet(ctx, core.FormatRedisHash(seconds), url.Id, url).Err(); err != nil {
			return fmt.Errorf("caching url %d: %w", url.Id, err)
		}
		loaded++
		return nil
	})
	if err != nil {
		return err
	}

	o.Logger.Info("loaded monitored urls into redis", "count", loaded)
	return nil
}
