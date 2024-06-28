package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

var DynamicLabels = map[string]any{"hosts": nil, "vars": nil}

type PlaybookError struct {
	message string
}

func (e PlaybookError) Error() string {
	return fmt.Sprintf("playbook error: %s", e.message)
}

type VerificationError struct {
	message string
}

func (e VerificationError) Error() string {
	return fmt.Sprintf("verification error: %s", e.message)
}

type PlaybookSource struct {
	stdin bool
	path  string
}

// DirtyPlaybook object can be used to manipulate and (un)marshal its data.
type DirtyPlaybook yaml.MapSlice

// CleanPlaybook does not contain dynamic elements.
type CleanPlaybook yaml.MapSlice

func UnmarshalPlaybook(playbook []byte) (DirtyPlaybook, error) {
	var data []DirtyPlaybook
	if err := yaml.Unmarshal(playbook, &data); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, PlaybookError{"playbook contains no data"}
	}
	if len(data) > 1 {
		return nil, PlaybookError{"input cannot contain more than one playbook"}
	}
	return data[0], nil
}

// GetExclusions extracts dynamic keys that are meant to be excluded from the playbook hash.
func (p *DirtyPlaybook) GetExclusions() ([][]string, error) {
	rawExclusions := ""
	for _, item := range *p {
		if item.Key.(string) != "vars" {
			continue
		}
		vars := item.Value.(DirtyPlaybook)
		for _, pair := range vars {
			if pair.Key.(string) != "insights_signature_exclude" {
				continue
			}
			rawExclusions = pair.Value.(string)
			break
		}
	}

	if rawExclusions == "" {
		return nil, PlaybookError{"playbook doesn't contain key 'insights_signature_exclude'"}
	}

	var exclusions [][]string
	for _, exclusion := range strings.Split(rawExclusions, ",") {
		exclusionBits := strings.TrimPrefix(exclusion, "/")
		exclusions = append(exclusions, strings.Split(exclusionBits, "/"))
	}
	return exclusions, nil
}

func (p *DirtyPlaybook) Clean() (*CleanPlaybook, error) {
	exclusions, err := p.GetExclusions()
	if err != nil {
		return nil, err
	}

	clean := CleanPlaybook{}
	for _, directValue := range *p {
		directValueName := directValue.Key.(string)
		skipDirectValue := false

		// exclusion of top-level/direct values
		for _, exclusion := range exclusions {
			if directValueName == exclusion[0] && len(exclusion) == 1 {
				skipDirectValue = true
			}
		}
		if skipDirectValue {
			slog.Debug("excluding", slog.String("path", directValue.Key.(string)))
			continue
		}

		// exclusion of nested values
		// TODO

		clean = append(clean, directValue)
	}
	return &clean, nil
}

// Marshall takes in the playbook and marshals it into a string
// as per the requirements of the hashing scheme.
func (p *CleanPlaybook) Marshall() ([]byte, error) {
	// TODO dict as ...
	// TODO list as ...
	// TODO ...

	return []byte{}, nil
}

// cleanPlaybook deletes specific keys from the playbook.
//
// Some parts of the Ansible playbook are dynamic (e.g. hosts, the signature) and cannot be
// signed, because the signature would be unique for every playbook that is generated.
//
// This function removes these variable sections.
func cleanPlaybook(p *DirtyPlaybook) (*CleanPlaybook, error) {
	playbook := CleanPlaybook{}
	for _, v := range *p {
		playbook = append(playbook, yaml.MapItem{Key: v.Key, Value: v.Value})
	}

	for _, exclusion := range []string{} {
		var elements []string
		for _, element := range strings.Split(exclusion, "/") {
			if len(element) == 0 {
				continue
			}
			elements = append(elements, element)
		}

		// elements[0] has to be key explicitly allowed to be excluded
		if _, ok := DynamicLabels[elements[0]]; !ok {
			return nil, VerificationError{
				fmt.Sprintf("key '%s' is not allowed to be excluded", exclusion),
			}
		}

		// simple key deletion
		if len(elements) == 1 {
			slog.Debug("deleted dynamic key", "key", exclusion)
		}
		// nested key deletion
		if len(elements) == 2 {
			slog.Debug("deleted dynamic key", "key", exclusion)
		}
	}

	return &playbook, nil
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
	rawPlaybook, err := readPlaybook(source)
	if err != nil {
		slog.Error("error getting playbook content", slog.Any("error", err))
		return
	}

	// Convert it into object
	dirty, err := UnmarshalPlaybook(rawPlaybook)
	if err != nil {
		slog.Error("could not parse playbook", slog.Any("error", err))
		return
	}

	// Delete dynamic elements
	clean, err := dirty.Clean()
	if err != nil {
		slog.Error("could not clean playbook", slog.Any("error", err))
	}
	fmt.Println("clean", clean)

	// Serialize it

	// Create a hash

	// Verify the hash

	// Print the original playbook
	return
}

// readPlaybook reads the playbook verifier from either stdin or from a file.
func readPlaybook(source PlaybookSource) ([]byte, error) {
	var rawPlaybook []byte
	if source.stdin {
		playbook, err := io.ReadAll(os.Stdin)
		if err != nil {
			slog.Error("could not read playbook from stdin", slog.Any("error", err))
			return []byte{}, err
		}
		rawPlaybook = playbook
	} else {
		playbook, err := os.ReadFile(source.path)
		if err != nil {
			slog.Error("could not read playbook from file", slog.Any("error", err))
			return []byte{}, err
		}
		rawPlaybook = playbook
	}

	slog.Debug("playbook obtained", slog.Int("bytes", len(rawPlaybook)))
	return rawPlaybook, nil
}
