package main

import (
	"context"
	"github.com/DevloperAmanSingh/watcher/cmd/commands"
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/DevloperAmanSingh/watcher/logger"
	"github.com/urfave/cli/v3"
	"log"
	"os"
)

var command commands.CommandContainer

func main() {
	ctx := context.Background()
	ctx = context.WithoutCancel(ctx)

	if err := env.Load(".env"); err != nil {
		log.Fatal(err)
	}

	newLogger := logger.New()
	cmd := &cli.Command{
		Name:     "Watcher",
		Usage:    "A HTTP monitoring service written in Golang",
		Flags:    []cli.Flag{},
		Commands: command.Initiate(newLogger),
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
