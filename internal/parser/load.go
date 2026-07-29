package parser

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// Load reads and decodes a YAML file from the regex database.
func Load[T any](fsys fs.FS, name string, out *T) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("devicedetector: reading %s: %w", name, err)
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("devicedetector: parsing %s: %w", name, err)
	}

	return nil
}

// OrderedEntry is a key/value pair of an OrderedMap.
type OrderedEntry[T any] struct {
	Key   string
	Value T
}

// OrderedMap decodes a YAML mapping while preserving document order.
// Several database files (notably the device brand map) rely on entry order
// for correct matching, which a plain Go map would destroy.
type OrderedMap[T any] struct {
	Entries []OrderedEntry[T]
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (m *OrderedMap[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("devicedetector: expected mapping node, got kind %d", node.Kind)
	}

	m.Entries = make([]OrderedEntry[T], 0, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		var entry OrderedEntry[T]

		if err := node.Content[i].Decode(&entry.Key); err != nil {
			return err
		}

		if err := node.Content[i+1].Decode(&entry.Value); err != nil {
			return err
		}

		m.Entries = append(m.Entries, entry)
	}

	return nil
}
