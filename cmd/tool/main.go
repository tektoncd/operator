package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:          "operator-tool",
		Short:        "Tooling to manage the operator",
		Long:         `This is a tool to help maintaining this operator`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	ioStreams := cli.NewIOStreams()

	cmd.AddCommand(BumpCommand(ioStreams))
	cmd.AddCommand(CheckCommand(ioStreams))
	cmd.AddCommand(ComponentVersionCommand(ioStreams))

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
