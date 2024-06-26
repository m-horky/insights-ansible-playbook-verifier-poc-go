package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

var VerificationError = errors.New("verification failed")

// FIXME We'll need to use stable map that keeps the insertion order.
// E.g. https://pkg.go.dev/github.com/wk8/go-ordered-map.

type PlaybookType = []map[string]any

type PlaybookSource struct {
	stdin bool
	path  string
}

// NewPlaybookSource detects the location for the playbook.
//
// If environment variable `PLAYBOOK_SOURCE` is set, it is interpreted as a path on a filesystem.
// If the variable is empty or not set, the source will be set to standard input.
func NewPlaybookSource() PlaybookSource {
	path := os.Getenv("PLAYBOOK_SOURCE")
	source := PlaybookSource{stdin: path == "", path: path}
	slog.Debug("determined playbook source:", slog.Any("source", source))
	return source
}

func (s PlaybookSource) String() string {
	if s.stdin {
		return "stdin"
	}
	return s.path
}

func main() {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// Load playbook from stdin
	source := NewPlaybookSource()
	rawPlaybook, err := getPlaybookContent(source)
	if err != nil {
		slog.Error("error getting playbook content:", slog.Any("error", err))
		return
	}

	playbook, err := parsePlaybook(rawPlaybook)
	if err != nil {
		slog.Error("could not parse playbook:", slog.Any("error", err))
		return
	}

	for k, v := range playbook[0] {
		fmt.Println(k, v)
	}

	// Verify its signature
	// Emit playbook to stdout
}

// getPlaybookContent reads the playbook verifier from either stdin or from a file.
func getPlaybookContent(source PlaybookSource) ([]byte, error) {
	var rawPlaybook []byte
	if source.stdin {
		readPlaybook, err := io.ReadAll(os.Stdin)
		if err != nil {
			slog.Error("could not read playbook from stdin", slog.Any("error", err))
			return []byte{}, err
		}
		rawPlaybook = readPlaybook
	} else {
		readPlaybook, err := os.ReadFile(source.path)
		if err != nil {
			slog.Error("could not read playbook from file", slog.Any("error", err))
			return []byte{}, err
		}
		rawPlaybook = readPlaybook
	}

	slog.Debug("playbook obtained", slog.Int("bytes", len(rawPlaybook)))
	return rawPlaybook, nil
}

func parsePlaybook(playbook []byte) (PlaybookType, error) {
	data := PlaybookType{}
	err := yaml.Unmarshal(playbook, &data)
	return data, err
}
