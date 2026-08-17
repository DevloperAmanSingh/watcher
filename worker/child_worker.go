package worker

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/DevloperAmanSingh/watcher/supervisor"
	"net/http"
	"time"
)

type ChildWorker struct {
	Id           int
	Ctx          context.Context
	ParentWorker *ParentWorker
}

func NewChildWorker(ctx context.Context, id int, parentWorker *ParentWorker) *ChildWorker {
	return &ChildWorker{
		Id:           id,
		Ctx:          ctx,
		ParentWorker: parentWorker,
	}
}

func (cw *ChildWorker) Start() {
	defer cw.ParentWorker.ChildWorkerPoolWaitGroup.Done()
	for {
		select {
		case <-cw.Ctx.Done():
			return

		case urlIds, ok := <-cw.ParentWorker.WorkPool:
			if !ok {
				fmt.Printf("Worker %d: WorkPool with parent %v interval closed, shutting down\n", cw.Id, cw.ParentWorker.Interval)
				return
			}
			fmt.Printf("Worker %d with parent %v interval picked up chunk of %d URLs\n", cw.Id, cw.ParentWorker.Interval, len(urlIds))

			for _, urlId := range urlIds {
				fmt.Printf("Worker %d with parent %v interval processing: %s\n", cw.Id, cw.ParentWorker.Interval, urlId)
				cw.Work(urlId)
			}
			fmt.Printf("Worker %d with parent %v interval completed chunk\n", cw.Id, cw.ParentWorker.Interval)
		}
	}
}

func (cw *ChildWorker) Work(urlId string) {
	client := &http.Client{
		Timeout: time.Duration(env.FetchInt("HTTP_REQUEST_TIMEOUT", 5)) * time.Second,
	}
	var url database.Url
	val, err := cw.ParentWorker.RedisClient.HGet(cw.Ctx, core.FormatRedisHash(cw.ParentWorker.Interval), urlId).Bytes()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = url.UnmarshalBinary(val)
	if err != nil {
		return
	}

	request, err := http.NewRequest(url.HttpMethod.ToMethod(), url.Url, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	resp, err := client.Do(request)
	if err != nil {
		fmt.Printf("client error: %v", err)
		task := supervisor.Task{
			Healthy: false,
			Url:     url.Url,
			UrlId:   url.Id,
		}
		cw.ParentWorker.Supervisor.WorkPool <- task
		return
	}
	defer resp.Body.Close()
	fmt.Printf("Worker %d with parent %v interval tried monitoring %v and returned %v \n", cw.Id, cw.ParentWorker.Interval, url, resp.StatusCode)
	task := supervisor.Task{
		UrlId:   url.Id,
		Healthy: resp.StatusCode > 199 && resp.StatusCode < 300,
		Url:     url.Url,
	}

	cw.ParentWorker.Supervisor.WorkPool <- task
	return
}
