package tzlocal

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// EnvTZ returns the timezone selected by TZ if it is set. Go treats invalid values as UTC.
func EnvTZ() (string, bool) {
	if name, ok := os.LookupEnv("TZ"); ok {
		// Go accepts an optional leading colon in TZ.
		name = strings.TrimPrefix(name, ":")
		// Go treats blank as UTC
		if name == "" {
			return "UTC", true
		}
		_, err := time.LoadLocation(name)
		// Go treats invalid as UTC
		if err != nil {
			return "UTC", true
		}
		return name, true
	}
	return "", false
}

// RuntimeTZ get the full timezone name of the local machine
func RuntimeTZ() (string, error) {
	// Get the timezone from the TZ env variable
	if name, ok := EnvTZ(); ok {
		return name, nil
	}

	// Get the timezone from the system file
	name, err := LocalTZ()
	if err != nil {
		return "", fmt.Errorf("failed to get local machine timezone: %w", err)
	}

	return name, err
}
