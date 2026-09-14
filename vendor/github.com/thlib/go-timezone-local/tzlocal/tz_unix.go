//go:build !windows
// +build !windows

package tzlocal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const localZoneFile = "/etc/localtime" // symlinked file - set by OS

func inferFromPath(p string) (string, error) {
	for _, base := range []string{"/zoneinfo/", "/zoneinfo.default/"} {
		i := strings.LastIndex(p, base)
		if i >= 0 {
			return p[i+len(base):], nil
		}
	}

	return "", fmt.Errorf("cannot infer timezone name from path: %q", p)
}

func localTZ(localZoneFile string) (string, error) {
	target, err := os.Readlink(localZoneFile)
	if err != nil {
		fi, statErr := os.Lstat(localZoneFile)
		if statErr != nil {
			return "", fmt.Errorf("failed to stat %q: %w", localZoneFile, statErr)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return "", fmt.Errorf("%q is not a symlink - cannot infer name", localZoneFile)
		}
		return "", err
	}

	if !filepath.IsAbs(target) {
		dir, err := filepath.EvalSymlinks(filepath.Dir(localZoneFile))
		if err != nil {
			return "", err
		}
		target = filepath.Join(dir, target)
	}

	p, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}

	return inferFromPath(p)
}

// LocalTZ gets the timezone name by resolving the /etc/localtime symlink.
func LocalTZ() (string, error) {
	return localTZ(localZoneFile)
}
