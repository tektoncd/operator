package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type component struct {
	Github  string `json:"github"`
	Version string `json:"version"`
}

func ReadCompoments(filename string) (map[string]component, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	components := map[string]component{}
	if err := yaml.Unmarshal(data, &components); err != nil {
		return nil, err
	}
	return components, nil
}

func writeComponents(filename string, components map[string]component) error {
	data, err := yaml.Marshal(components)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o644)
}

func ComponentVersionCommand(ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component-version",
		Short: "Prints the version of a component from a components file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("Requires at least 2 argument")
			}
			filename := args[0]
			return componentVersion(filename, args[1:], ioStreams.Out)
		},
	}
	return cmd
}

func componentVersion(filename string, args []string, out io.Writer) error {
	if len(args) == 0 || len(args) > 1 {
		return fmt.Errorf("Need one and only one argument, the component name")
	}
	component := args[0]
	components, err := ReadCompoments(filename)
	if err != nil {
		return err
	}
	c, ok := components[component]
	if !ok {
		return fmt.Errorf("Component %s not found", component)
	}
	fmt.Fprint(out, c.Version)
	return nil
}
