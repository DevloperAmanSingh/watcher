// Package env resolves configuration from the process environment.
//
// The process environment is always the source of truth. A dotenv file, if
// present, only supplies defaults for keys that are not already set, which
// keeps local development convenient without making the binary depend on a
// file that no container platform will ever provide.
package env

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Load reads defaults from the named dotenv file into the process
// environment. Keys already set in the environment are left untouched.
//
// A missing file is not an error: production deployments inject
// configuration directly and have no dotenv file to read.
func Load(filename string) error {
	err := godotenv.Load(filename)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("reading %s: %w", filename, err)
}

// FetchString returns the value of key, or the fallback when key is unset or
// empty. It panics when key is unset and no fallback is provided, because a
// missing required setting should fail at startup rather than at first use.
func FetchString(key string, fallback ...string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
}

// FetchInt returns the value of key parsed as an integer.
//
// An unset or empty key yields the fallback, or panics when none is given. A
// key that is set but unparseable always panics: silently substituting a
// default for a typo'd value hides misconfiguration until it causes damage in
// production.
func FetchInt(key string, fallback ...int) int {
	value, ok := lookup(key)
	if !ok {
		if len(fallback) > 0 {
			return fallback[0]
		}
		panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s is not an integer: %q", key, value))
	}
	return parsed
}

// FetchBool returns the value of key parsed as a boolean, accepting the forms
// strconv.ParseBool understands (1, t, true, 0, f, false, ...).
func FetchBool(key string, fallback ...bool) bool {
	value, ok := lookup(key)
	if !ok {
		if len(fallback) > 0 {
			return fallback[0]
		}
		panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s is not a boolean: %q", key, value))
	}
	return parsed
}

// FetchDuration returns the value of key parsed as a Go duration string, such
// as "5s" or "2m30s".
func FetchDuration(key string, fallback ...time.Duration) time.Duration {
	value, ok := lookup(key)
	if !ok {
		if len(fallback) > 0 {
			return fallback[0]
		}
		panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s is not a duration: %q", key, value))
	}
	return parsed
}

// lookup reports whether key is set to a non-empty value. An empty value is
// treated as unset so that an exported-but-blank variable falls back to the
// default rather than failing to parse.
func lookup(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}
