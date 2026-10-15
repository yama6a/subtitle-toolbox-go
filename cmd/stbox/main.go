// Command stbox converts, checks and fixes subtitle files.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
		_, _ = fmt.Fprintln(stdout, buildVersion())
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "usage: stbox --version")
	return 2
}
