package commands

import (
	"context"
	"fmt"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/DevloperAmanSingh/watcher/orchestrator"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"
	"time"
)

type GuardCommand struct {
	*BaseCommand
}

func (mc *GuardCommand) Action(ctx context.Context, cmd CommandContext) error {
	return Init(ctx, mc.Log)
}

func NewGuardCommand(logger *slog.Logger) *GuardCommand {
	return &GuardCommand{
		BaseCommand: &BaseCommand{
			name:    "guard",
			aliases: []string{"g"},
			usage:   "Start the watcher monitoring process.",
			args:    []ArgumentContext{},
			flags:   []FlagContext{},
			Log:     logger,
		},
	}
}

func Init(ctx context.Context, logger *slog.Logger) error {
	redisClient := InitiateRedis(ctx, logger)
	pool := InitiateDB(ctx, logger)
	fmt.Println("Watcher is running")
	return initiateOrchestrator(ctx, redisClient, pool)
}

func InitiateRedis(ctx context.Context, logger *slog.Logger) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:         env.FetchString("REDIS_HOST", "127.0.0.1:6379"),
		Password:     env.FetchString("REDIS_PASS", ""),
		DB:           env.FetchInt("REDIS_DB", 0),
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	err := redisClient.Ping(ctx).Err()
	if err != nil {
		logger.Error("redis connection failed",
			"error", err, "addr", env.FetchString("REDIS_HOST", "127.0.0.1:6379"))
		panic("redis connection failed")
	}
	return redisClient
}

func InitiateDB(ctx context.Context, logger *slog.Logger) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://%v:%v@%v:%v/%v",
		env.FetchString("DB_USER"),
		env.FetchString("DB_PASSWORD", ""),
		env.FetchString("DB_HOST"),
		env.FetchString("DB_PORT", "5432"),
		env.FetchString("DB_DATABASE")))
	if err != nil {
		panic(fmt.Sprintf("pgxpool connection failed: %v", err))
	}
	if err := pool.Ping(ctx); err != nil {
		logger.Error("database connection failed",
			"error", err,
			"host", env.FetchString("DB_HOST"),
			"database", env.FetchString("DB_DATABASE"))
		os.Exit(1)
	}

	fmt.Println("Connected to PostgreSQL database!")
	return pool
}

func initiateOrchestrator(ctx context.Context, redisClient *redis.Client, pool *pgxpool.Pool) error {
	newOrchestrator := orchestrator.NewOrchestrator(ctx, redisClient, pool)
	newOrchestrator.Supervisor.Activate()
	intervals := []enums.MonitoringFrequency{
		enums.TenSeconds,
		enums.ThirtySeconds,
		enums.OneMinute,
		enums.FiveMinutes,
		enums.ThirtyMinutes,
		enums.OneHour,
		enums.TwelveHours,
		enums.TwentyFourHours,
	}

	newOrchestrator.AddIntervals(intervals)
	return newOrchestrator.Start()
}
