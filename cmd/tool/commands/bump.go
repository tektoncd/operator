package commands

import (
	"fmt"
	"sort"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

var bugfix bool

func BumpCommand(ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bump",
		Short: "Bump components version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Requires 1 argument")
			}
			filename := args[0]
			return bump(filename, bugfix)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.Flags().BoolVar(&bugfix, "bugfix", false, "Only update bugfix versions of components")
	return cmd
}

func bump(filename string, bugfix bool) error {
	newComponents := map[string]component{}
	components, err := ReadCompoments(filename)
	if err != nil {
		return err
	}
	for name, component := range components {
		newComponent, err := bumpComponent(name, component, bugfix)
		if err != nil {
			return err
		}
		newComponents[name] = newComponent
	}
	return writeComponents(filename, newComponents)
}

func bumpComponent(name string, c component, bugfix bool) (component, error) {
	newVersion := c.Version
	newerVersions, err := checkComponentNewerVersions(c, bugfix)
	if err != nil {
		return component{}, err
	}
	if len(newerVersions) > 0 {
		// Get the latest one
		sort.Sort(newerVersions) // sort just in case
		newVersion = "v" + newerVersions[len(newerVersions)-1].String()
	}
	return component{
		Github:  c.Github,
		Version: newVersion,
	}, nil
}
