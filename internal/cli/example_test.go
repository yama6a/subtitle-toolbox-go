package cli_test

import (
	"context"
	"fmt"
	"os"

	"github.com/yama6a/subtitle-toolbox-go/internal/cli"
)

func ExampleRun() {
	err := cli.Run(context.Background(), []string{"--version"}, cli.Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stdout, Version: "1.2.3",
	})
	fmt.Println(cli.ExitCode(err))
	// Output:
	// 1.2.3
	// 0
}

func ExampleExitCode() {
	err := cli.Run(context.Background(), []string{"merge"}, cli.Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stdout, Version: "1.2.3",
	})
	fmt.Println(err, cli.ExitCode(err))
	// Output:
	// Error: Unknown command "merge".
	// Run "stbox help" for the usage.
	// exit code 2 2
}
