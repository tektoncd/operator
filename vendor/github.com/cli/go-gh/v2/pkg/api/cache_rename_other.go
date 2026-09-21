//go:build !windows

package api

import "os"

func renameCacheFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
