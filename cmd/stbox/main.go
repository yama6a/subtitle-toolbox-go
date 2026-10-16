// Command stbox converts, checks and fixes subtitle files.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/yama6a/subtitle-toolbox-go/internal/cli"
)

func main() {
	if err := run(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}

func run() error {
	ctx, stop := notifyContext()
	defer stop()

	return cli.Run(ctx, os.Args[1:], cli.Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Getenv: os.Getenv, Version: buildVersion(),
	})
}

// notifyContext ends ctx on the first SIGINT or SIGTERM, and the run stops before its next input. Then the default
// handling returns, so a second signal ends the process, also while a read of standard input blocks.
func notifyContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)
	return ctx, stop
}
