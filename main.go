package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
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

func UnmarshalPlaybook(playbook []byte) (yaml.MapSlice, error) {
	var data []yaml.MapSlice
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

// GetPlaybookExclusions extracts dynamic keys that are meant to be excluded from the playbook hash.
func GetPlaybookExclusions(p *yaml.MapSlice) ([][]string, error) {
	rawExclusions := ""
	for _, item := range *p {
		if item.Key.(string) != "vars" {
			continue
		}
		vars := item.Value.(yaml.MapSlice)
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

func CleanPlaybook(p *yaml.MapSlice) (*yaml.MapSlice, error) {
	exclusions, err := GetPlaybookExclusions(p)
	if err != nil {
		return nil, err
	}

	clean := yaml.MapSlice{}
	for _, directValue := range *p {
		directValueName := directValue.Key.(string)
		skipDirectValue := false

		if reflect.TypeOf(directValue.Value) == reflect.TypeOf(yaml.MapSlice{}) {
			// nested exclusion
			newDirectValue := yaml.MapSlice{}

			for _, nestedValue := range directValue.Value.(yaml.MapSlice) {
				nestedValueName := nestedValue.Key.(string)
				skipNestedValue := false

				for _, exclusion := range exclusions {
					if directValueName == exclusion[0] && len(exclusion) == 2 && nestedValueName == exclusion[1] {
						skipNestedValue = true
					}
				}

				if skipNestedValue {
					slog.Info("excluding nested", slog.String("path", directValueName+"/"+nestedValueName))
					continue
				}

				slog.Debug("including nested", slog.String("path", directValueName+"/"+nestedValueName))
				newDirectValue = append(newDirectValue, yaml.MapItem{Key: nestedValue.Key, Value: nestedValue.Value})
			}

			directValue = yaml.MapItem{Key: directValue.Key, Value: newDirectValue}
		} else {
			// simple exclusion
			for _, exclusion := range exclusions {
				if directValueName == exclusion[0] && len(exclusion) == 1 {
					skipDirectValue = true
				}
			}
			if skipDirectValue {
				slog.Info("excluding direct", slog.String("path", directValueName))
				continue
			}
		}

		slog.Debug("including direct", slog.String("path", directValueName))
		clean = append(clean, directValue)
	}

	slog.Debug("playbook cleaned")
	return &clean, nil
}

// NewPlaybookSource detects the location for the playbook.
//
// If environment variable `PLAYBOOK_SOURCE` is set, it is interpreted as a path on a filesystem.
// If the variable is empty or not set, the source will be set to standard input.
func NewPlaybookSource() PlaybookSource {
	path := os.Getenv("PLAYBOOK_SOURCE")
	source := PlaybookSource{stdin: path == "", path: path}
	slog.Debug("determined playbook source", slog.Any("source", source))
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
	clean, err := CleanPlaybook(&dirty)
	if err != nil {
		slog.Error("could not clean playbook", slog.Any("error", err))
	}

	// Serialize it
	serialized, err := MarshallPlaybook(clean)
	if err != nil {
		slog.Error("could not serialize playbook", slog.Any("error", err))
	}
	fmt.Println(string(serialized))

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

	slog.Debug("playbook loaded")
	return rawPlaybook, nil
}
