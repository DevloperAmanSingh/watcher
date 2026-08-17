package commands

import (
	"context"
	"github.com/DevloperAmanSingh/watcher/core"
	"github.com/DevloperAmanSingh/watcher/database"
	"github.com/DevloperAmanSingh/watcher/enums"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v3"
	"log/slog"
)

type Command interface {
	Name() string
	Action(ctx context.Context, cmd CommandContext) error
	Aliases() []string
	Usage() string
	Arguments() []ArgumentContext
	Flags() []FlagContext
}

type CommandContainer struct {
	Commands []Command
}

func (cc *CommandContainer) Register(command Command) {
	cc.Commands = append(cc.Commands, command)
	return
}

func (cc *CommandContainer) RegisterAll(logger *slog.Logger) {
	cc.Register(NewAnalysisCommand(logger))
	cc.Register(NewListCommand(logger))
	cc.Register(NewRemoveCommand(logger))
	cc.Register(NewAddCommand(logger))
	cc.Register(NewGuardCommand(logger))
}

func (cc *CommandContainer) Initiate(logger *slog.Logger) []*cli.Command {
	cc.RegisterAll(logger)
	var commands []*cli.Command
	for _, command := range cc.Commands {
		var arguments []cli.Argument
		var flags []cli.Flag

		for _, argument := range command.Arguments() {
			var transformedArgument cli.Argument
			if argument.Type == enums.Int {
				transformedArgument = &cli.IntArg{
					Name:      argument.Name,
					UsageText: argument.Usage,
				}
			} else {
				transformedArgument = &cli.StringArg{
					Name:      argument.Name,
					UsageText: argument.Usage,
				}
			}
			arguments = append(arguments, transformedArgument)
		}

		for _, flag := range command.Flags() {
			var transformedFlag cli.Flag
			if flag.Type == enums.Int {
				transformedFlag = &cli.IntFlag{
					Name:  flag.Name,
					Usage: flag.Usage,
					Value: flag.Default.(int),
				}
			} else {
				transformedFlag = &cli.StringFlag{
					Name:  flag.Name,
					Usage: flag.Usage,
					Value: flag.Default.(string),
				}
			}
			flags = append(flags, transformedFlag)
		}

		commands = append(commands, &cli.Command{
			Name:    command.Name(),
			Usage:   command.Usage(),
			Aliases: command.Aliases(),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				wrapped := &UrfaveContext{cmd: cmd}
				return command.Action(ctx, wrapped)
			},
			Arguments: arguments,
			Flags:     flags,
		})
	}
	return commands
}

type CommandContext interface {
	String(name string) string
	Int(name string) int
	Args() []string
	BoolFlag(name string) bool
	IntFlag(name string) int
	StringFlag(name string) string
}

type ArgumentContext struct {
	Name    string
	Usage   string
	Type    enums.ArgumentType
	Default interface{}
}

type FlagContext struct {
	Name    string
	Usage   string
	Type    enums.ArgumentType
	Default interface{}
}
type UrfaveContext struct {
	cmd *cli.Command
}

func (u *UrfaveContext) String(name string) string {
	return u.cmd.StringArg(name)
}

func (u *UrfaveContext) Int(name string) int {
	return u.cmd.IntArg(name)
}

func (u *UrfaveContext) Args() []string {
	return u.cmd.Args().Slice()
}

func (u *UrfaveContext) BoolFlag(name string) bool {
	return u.cmd.Bool(name)
}

func (u *UrfaveContext) StringFlag(name string) string {
	return u.cmd.String(name)
}

func (u *UrfaveContext) IntFlag(name string) int {
	return u.cmd.Int(name)
}

type BaseCommand struct {
	name    string
	aliases []string
	usage   string
	args    []ArgumentContext
	flags   []FlagContext
	Log     *slog.Logger
}

func (b *BaseCommand) Name() string                 { return b.name }
func (b *BaseCommand) Aliases() []string            { return b.aliases }
func (b *BaseCommand) Usage() string                { return b.usage }
func (b *BaseCommand) Arguments() []ArgumentContext { return b.args }
func (b *BaseCommand) Flags() []FlagContext         { return b.flags }

func RefreshRedisInterval(ctx context.Context, redisClient *redis.Client, pool *pgxpool.Pool, frequency enums.MonitoringFrequency) error {
	urls, err := database.NewUrlRepository(pool).FetchAll(ctx, 10, 0, database.UrlQueryFilter{
		Frequency: frequency,
	})
	if err != nil {
		return err
	}

	redisClient.Del(ctx, core.FormatRedisList(frequency.ToSeconds()))

	for _, url := range urls {
		redisClient.LPush(ctx, core.FormatRedisList(url.MonitoringFrequency.ToSeconds()), url.Id)
		redisClient.HSet(ctx, core.FormatRedisHash(url.MonitoringFrequency.ToSeconds()), url.Id, url)
	}
	return nil
}
