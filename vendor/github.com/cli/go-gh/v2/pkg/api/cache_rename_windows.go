package api

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func renameCacheFile(oldPath string, newPath string) error {
	return retryCacheRename(func() (error, bool) {
		err := os.Rename(oldPath, newPath)
		return err, isRetryableCacheRenameError(err)
	})
}

type retryableFn func() (err error, retryable bool)

func retryCacheRename(rename retryableFn) error {
	// Caching is best-effort: allow brief Windows sharing conflicts to clear
	// without holding up an API response indefinitely.
	deadline := time.Now().Add(100 * time.Millisecond)
	for delay := time.Millisecond; ; delay *= 2 {
		err, retryable := rename()
		if err == nil {
			return nil
		}

		if !retryable {
			return err
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		time.Sleep(min(delay, remaining))
	}
}

func isRetryableCacheRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
