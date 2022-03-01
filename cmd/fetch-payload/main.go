package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"
)

var (
	filename  string
	targetDir string
	platform  string
)

func main() {
	flag.StringVar(&filename, "filename", "", "components configuration to load")
	flag.StringVar(&targetDir, "target", ".", "target folder where to put fetched payloads")
	flag.StringVar(&platform, "platform", "kubernetes", "platform payload to fetch")
	flag.Parse()

	if err := fetchPayload(context.Background(), filename, targetDir, platform); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func fetchPayload(ctx context.Context, filename, targetDir, platform string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	components := map[string]component{}
	if err := yaml.Unmarshal(data, &components); err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	for name, component := range components {

		// Force scope
		name := name
		component := component
		component.Name = name

		g.Go(func() error {
			return fetchComponent(ctx, name, component, targetDir, platform)
		})
	}
	return g.Wait()
}

func fetchComponent(ctx context.Context, name string, c component, targetDir, platform string) error {
	// fmt.Printf("component: %s\n", name)
	// fmt.Printf("version: %s\n", c.Version)
	if !c.toFetch(platform) {
		fmt.Fprintf(os.Stderr, "skip: component %s on platform %s\n", name, platform)
		return nil
	}
	g, ctx := errgroup.WithContext(ctx)
	for i, t := range c.GetTargets() {
		i := i
		t := t
		g.Go(func() error {
			path := filepath.Join(targetDir, "cmd", platform, "kodata", name, t.Prefix, normalizedVersion(c.Version), fmt.Sprintf("0%d-%s.yaml", i, t.Name))
			if filepath.IsAbs(t.Url) || !strings.HasPrefix(t.Url, "http") {
				return copyFile(t.Url, path)
			}
			return download(t.Url, path)
		})
	}
	return g.Wait()
}

func download(url, path string) error {
	fmt.Printf("Download %s to %s\n", url, path)
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
		return fmt.Errorf("Error fetch %s: HTTP error %d", url, resp.StatusCode)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(src, target string) error {
	fmt.Printf("Copy %s to %s\n", src, target)
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	// Create the file
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, source)
	return err
}
