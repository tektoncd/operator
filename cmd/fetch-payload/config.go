package main

import (
	"fmt"
	"strings"
)

const (
	nightlyURIFormat       = "https://storage.googleapis.com/tekton-releases-nightly/%s/latest/%s.yaml"
	releaseLatestURIFormat = "https://storage.googleapis.com/tekton-releases/%s/latest/%s.yaml"
	releasedURIFormat      = "https://storage.googleapis.com/tekton-releases/%s/previous/{{version}}/%s.yaml"
)

type component struct {
	Name      string
	Version   string
	Internal  string
	Targets   []target
	Platforms []string
}

func (c component) toFetch(platform string) bool {
	if len(c.Platforms) == 0 {
		// All platforms
		return true
	}
	for _, p := range c.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

func (c component) GetTargets() []target {
	targets := c.Targets
	if len(c.Targets) == 0 {
		// Default, generate one from version
		format := getFormat(c.Version)
		targets = []target{{
			Name: "release",
			Url:  fmt.Sprintf(format, c.Internal, "release"),
		}}
	}
	r := make([]target, len(targets))
	for i, t := range targets {
		r[i] = t.normalized(c.Internal, c.Version)
	}
	return r
}

func normalizedVersion(version string) string {
	if version == "" || version == "latest" {
		return "0.0.0-latest"
	} else if version == "nightly" {
		return "0.0.0-nightly"
	}
	return version
}

type target struct {
	Name   string
	Prefix string
	Url    string
}

func (t target) normalized(internal, version string) target {
	tname := t.Name
	tprefix := t.Prefix
	turl := t.Url
	if turl == "" {
		format := getFormat(version)
		turl = fmt.Sprintf(format, internal, tname)
	}
	return target{
		Name:   tname,
		Prefix: tprefix,
		Url:    turl,
	}
}

func getFormat(version string) string {
	var format string
	if version == "" || version == "latest" {
		format = releaseLatestURIFormat
	} else if version == "nightly" {
		format = nightlyURIFormat
	} else {
		format = strings.Replace(releasedURIFormat, "{{version}}", version, -1)
	}
	return format
}
