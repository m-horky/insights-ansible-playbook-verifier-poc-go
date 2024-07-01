package main

import (
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v2"
)

// MarshallPlaybook takes in the playbook and marshals it into a string
// as per the requirements of the hashing scheme.
func MarshallPlaybook(p *yaml.MapSlice) ([]byte, error) {
	slog.Debug("starting serialization")
	return marshallPlaybookMap(*p)
}

func marshallPlaybookItem(item any) ([]byte, error) {
	var value []byte

	switch item.(type) {
	case yaml.MapSlice:
		marshalled, err := marshallPlaybookMap(item.(yaml.MapSlice))
		if err != nil {
			return nil, err
		}
		value = marshalled
	case []any:
		marshalled, err := marshallPlaybookList(item.([]any))
		if err != nil {
			return nil, err
		}
		value = marshalled
	case bool:
		if item.(bool) {
			value = []byte("True")
		} else {
			value = []byte("False")
		}
	case string:
		value = []byte(fmt.Sprintf("'%s'", item.(string)))
	default:
		value = []byte(item.(string))
	}

	return value, nil
}

func marshallPlaybookMap(m yaml.MapSlice) ([]byte, error) {
	result := []byte("ordereddict([")

	for i, pair := range m {
		key := pair.Key.(string)

		value, err := marshallPlaybookItem(pair.Value)
		if err != nil {
			return nil, err
		}

		if i > 0 {
			result = append(result, []byte(", ")...)
		}

		result = append(result, []byte("('")...)
		result = append(result, key...)
		result = append(result, []byte("', ")...)
		result = append(result, value...)
		result = append(result, []byte(")")...)
	}

	result = append(result, []byte("])")...)
	return result, nil
}

func marshallPlaybookList(l []any) ([]byte, error) {
	result := []byte("[")

	for i, item := range l {
		value, err := marshallPlaybookItem(item)
		if err != nil {
			return nil, err
		}

		if i > 0 {
			result = append(result, []byte(", ")...)
		}
		result = append(result, value...)
	}

	result = append(result, []byte("]")...)
	return result, nil
}
