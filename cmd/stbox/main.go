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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return cli.Run(ctx, os.Args[1:], cli.Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Getenv: os.Getenv, Version: buildVersion(),
	})
}
