package commands

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/cli/go-gh"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func CheckCommand(ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check component versions (and if there is an upgrade needed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Requires 1 argument")
			}
			filename := args[0]
			return check(filename, bugfix, ioStreams.Out)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.Flags().BoolVar(&bugfix, "bugfix", false, "Only update bugfix versions of components")
	return cmd
}

func check(filename string, bugfix bool, out io.Writer) error {
	components, err := ReadComponents(filename)
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(context.Background())
	for name, component := range components {
		// Force scope
		name := name
		component := component

		g.Go(func() error {
			return checkComponent(ctx, name, component, bugfix, out)
		})
	}
	return g.Wait()
}

func checkComponent(ctx context.Context, name string, component component, bugfix bool, out io.Writer) error {
	newerVersion, err := checkComponentNewerVersions(component, bugfix)
	if err != nil {
		return err
	}
	if len(newerVersion) > 0 {
		fmt.Fprintf(out, "%s: %v\n", name, newerVersion)
	}

	return nil
}

func checkComponentNewerVersions(component component, bugfix bool) (semver.Collection, error) {
	sVersions, err := fetchVersions(component.Github)
	if err != nil {
		return nil, err
	}
	currentVersion, err := semver.NewVersion(component.Version)
	if err != nil {
		return nil, err
	}
	newerVersion, err := getNewerVersion(currentVersion, sVersions, bugfix)
	if err != nil {
		return nil, err
	}
	return newerVersion, nil
}

func fetchVersions(github string) (semver.Collection, error) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return nil, err
	}
	versions := []struct {
		Name    string
		TagName string `json:"tag_name"`
	}{}
	err = client.Get(fmt.Sprintf("repos/%s/releases", github), &versions)
	if err != nil {
		return nil, err
	}
	sVersions := semver.Collection([]*semver.Version{})
	for _, v := range versions {
		sVersion, err := semver.NewVersion(v.TagName)
		if err != nil {
			return nil, err
		}
		sVersions = append(sVersions, sVersion)
	}
	sort.Sort(sVersions)
	return sVersions, nil
}

func getNewerVersion(currentVersion *semver.Version, versions []*semver.Version, bugfix bool) (semver.Collection, error) {
	constraint := fmt.Sprintf("> %s", currentVersion)
	if bugfix {
		nextMinorVersion := currentVersion.IncMinor()
		constraint = fmt.Sprintf("> %s, < %s", currentVersion, nextMinorVersion.String())
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, err
	}
	newerVersion := semver.Collection([]*semver.Version{})
	for _, sv := range versions {
		if c.Check(sv) {
			newerVersion = append(newerVersion, sv)
		}
	}
	return newerVersion, nil
}
