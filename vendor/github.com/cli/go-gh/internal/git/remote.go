package git

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var remoteRE = regexp.MustCompile(`(.+)\s+(.+)\s+\((push|fetch)\)`)

type RemoteSet []*Remote

type Remote struct {
	Name     string
	FetchURL *url.URL
	PushURL  *url.URL
	Resolved string
	Host     string
	Owner    string
	Repo     string
}

func (r RemoteSet) Len() int      { return len(r) }
func (r RemoteSet) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r RemoteSet) Less(i, j int) bool {
	return remoteNameSortScore(r[i].Name) > remoteNameSortScore(r[j].Name)
}

func remoteNameSortScore(name string) int {
	switch strings.ToLower(name) {
	case "upstream":
		return 3
	case "github":
		return 2
	case "origin":
		return 1
	default:
		return 0
	}
}

func Remotes() (RemoteSet, error) {
	list, err := listRemotes()
	if err != nil {
		return nil, err
	}
	remotes := parseRemotes(list)
	setResolvedRemotes(remotes)
	sort.Sort(remotes)
	return remotes, nil
}

// Filter remotes by given hostnames, maintains original order.
func (rs RemoteSet) FilterByHosts(hosts []string) RemoteSet {
	filtered := make(RemoteSet, 0)
	for _, remote := range rs {
		for _, host := range hosts {
			if strings.EqualFold(remote.Host, host) {
				filtered = append(filtered, remote)
				break
			}
		}
	}
	return filtered
}

func listRemotes() ([]string, error) {
	stdOut, _, err := Exec("remote", "-v")
	if err != nil {
		return nil, err
	}
	return toLines(stdOut.String()), nil
}

func parseRemotes(gitRemotes []string) RemoteSet {
	remotes := RemoteSet{}
	for _, r := range gitRemotes {
		match := remoteRE.FindStringSubmatch(r)
		if match == nil {
			continue
		}
		name := strings.TrimSpace(match[1])
		urlStr := strings.TrimSpace(match[2])
		urlType := strings.TrimSpace(match[3])

		url, err := ParseURL(urlStr)
		if err != nil {
			continue
		}
		host, owner, repo, _ := RepoInfoFromURL(url)

		var rem *Remote
		if len(remotes) > 0 {
			rem = remotes[len(remotes)-1]
			if name != rem.Name {
				rem = nil
			}
		}
		if rem == nil {
			rem = &Remote{Name: name}
			remotes = append(remotes, rem)
		}

		switch urlType {
		case "fetch":
			rem.FetchURL = url
			rem.Host = host
			rem.Owner = owner
			rem.Repo = repo
		case "push":
			rem.PushURL = url
			if rem.Host == "" {
				rem.Host = host
			}
			if rem.Owner == "" {
				rem.Owner = owner
			}
			if rem.Repo == "" {
				rem.Repo = repo
			}
		}
	}
	return remotes
}

func setResolvedRemotes(remotes RemoteSet) {
	stdOut, _, err := Exec("config", "--get-regexp", `^remote\..*\.gh-resolved$`)
	if err != nil {
		return
	}
	for _, l := range toLines(stdOut.String()) {
		parts := strings.SplitN(l, " ", 2)
		if len(parts) < 2 {
			continue
		}
		rp := strings.SplitN(parts[0], ".", 3)
		if len(rp) < 2 {
			continue
		}
		name := rp[1]
		for _, r := range remotes {
			if r.Name == name {
				r.Resolved = parts[1]
				break
			}
		}
	}
}

func toLines(output string) []string {
	lines := strings.TrimSuffix(output, "\n")
	return strings.Split(lines, "\n")
}
