package nlu

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultFAQYAML contains the embedded data/faq.yaml content.
//
//go:embed data/faq.yaml
var DefaultFAQYAML []byte

// FAQEntry represents a single FAQ record containing an ID, question phrasings, and an answer.
type FAQEntry struct {
	ID        string   `yaml:"id"`
	Questions []string `yaml:"questions"`
	Answer    string   `yaml:"answer"`
}

// Validate checks that the list of FAQ entries and all their fields are non-empty.
func Validate(entries []FAQEntry) error {
	if len(entries) == 0 {
		return errors.New("faq dataset is empty")
	}

	for i, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			return fmt.Errorf("entry at index %d: id must not be empty", i)
		}
		if len(entry.Questions) == 0 {
			return fmt.Errorf("entry %q: questions list must not be empty", entry.ID)
		}
		for qIdx, q := range entry.Questions {
			if strings.TrimSpace(q) == "" {
				return fmt.Errorf("entry %q: question at index %d must not be empty", entry.ID, qIdx)
			}
		}
		if strings.TrimSpace(entry.Answer) == "" {
			return fmt.Errorf("entry %q: answer must not be empty", entry.ID)
		}
	}

	return nil
}

// Parse unmarshals YAML content into FAQ entries and validates all fields.
func Parse(data []byte) ([]FAQEntry, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("empty YAML data")
	}

	var entries []FAQEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal faq yaml: %w", err)
	}

	if err := Validate(entries); err != nil {
		return nil, fmt.Errorf("validate faq entries: %w", err)
	}

	return entries, nil
}

// LoadDefault parses and validates the embedded default FAQ entries.
func LoadDefault() ([]FAQEntry, error) {
	return Parse(DefaultFAQYAML)
}

// Load parses and validates FAQ entries from an optional io.Reader.
// If no reader is provided, it loads the embedded default FAQ entries.
func Load(r ...io.Reader) ([]FAQEntry, error) {
	if len(r) > 0 && r[0] != nil {
		data, err := io.ReadAll(r[0])
		if err != nil {
			return nil, fmt.Errorf("read faq: %w", err)
		}
		return Parse(data)
	}
	return LoadDefault()
}
