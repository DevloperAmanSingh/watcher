package worker

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/DevloperAmanSingh/watcher/supervisor"
	"github.com/redis/go-redis/v9"
	"log"
	"sync"
)

type ParentWorker struct {
	RedisClient              *redis.Client
	Signal                   chan bool
	Interval                 int
	Ctx                      context.Context
	WorkPool                 chan []string
	ChildWorkerPoolWaitGroup sync.WaitGroup
	Supervisor               *supervisor.Supervisor
}

func (pw *ParentWorker) Start() {
	maxChildWorkers := env.FetchInt("MAXIMUM_CHILD_WORKERS")
	if maxChildWorkers < 1 {
		panic("MAXIMUM_CHILD_WORKERS must be greater than 0")
	}
	pw.spawnChildWorkers(maxChildWorkers)
	go func() {
		for {
			select {
			case <-pw.Ctx.Done():
				return
			case <-pw.Signal:
				pw.Work()
			}
		}
	}()
}

func (pw *ParentWorker) Work() {
	urlLength, err := pw.RedisClient.LLen(pw.Ctx, core.FormatRedisList(pw.Interval)).Result()
	if err != nil {
		log.Println(err)
		return
	}

	if urlLength < 1 {
		return
	}

	//fetch all the ids in Redis
	urlIds, err := pw.RedisClient.LRange(pw.Ctx, core.FormatRedisList(pw.Interval), 0, urlLength).Result()
	fmt.Println(urlIds)
	if err != nil {
		log.Println(err)
		return
	}
	maxPoolSize := env.FetchInt("MAXIMUM_WORK_POOL_SIZE")
	for i := 0; i < len(urlIds); i += maxPoolSize {
		end := i + maxPoolSize
		if end > len(urlIds) {
			end = len(urlIds)
		}
		chunk := urlIds[i:end]
		pw.WorkPool <- chunk
	}
	return
}

func (pw *ParentWorker) spawnChildWorkers(maxChildWorkers int) {
	for i := 0; i < maxChildWorkers; i++ {
		pw.ChildWorkerPoolWaitGroup.Add(1)
		child := NewChildWorker(
			pw.Ctx,
			i+1,
			pw,
		)
		go child.Start()
	}
}

func NewParentWorker(ctx context.Context, redisClient *redis.Client, interval int, supervisor *supervisor.Supervisor) *ParentWorker {
	bufferSize := env.FetchInt("MAXIMUM_WORK_POOL_SIZE")
	return &ParentWorker{
		Ctx:                      ctx,
		RedisClient:              redisClient,
		Interval:                 interval,
		Signal:                   make(chan bool),
		WorkPool:                 make(chan []string, bufferSize),
		ChildWorkerPoolWaitGroup: sync.WaitGroup{},
		Supervisor:               supervisor,
	}
}
