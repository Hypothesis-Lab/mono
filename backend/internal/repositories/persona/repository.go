package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Persona struct {
	ID          string
	Version     string
	Source      string
	DisplayName string
	Dimensions  map[string]string
	Profile     string
}

type Query struct {
	Sources          []string
	DimensionFilters map[string][]string
}

type Repository struct {
	personas []*Persona
}

type manifest struct {
	Count int `json:"count"`
}

type datasetPersona struct {
	ID          string            `yaml:"persona_id"`
	Version     string            `yaml:"version"`
	Source      string            `yaml:"source"`
	DisplayName string            `yaml:"display_name"`
	Dimensions  map[string]string `yaml:"dimensions"`
}

func NewRepository(directory string) (*Repository, error) {
	manifestData, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read persona dataset manifest: %w", err)
	}

	var datasetManifest manifest
	if err := json.Unmarshal(manifestData, &datasetManifest); err != nil {
		return nil, fmt.Errorf("decode persona dataset manifest: %w", err)
	}

	paths, err := filepath.Glob(filepath.Join(directory, "persona_*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("list persona dataset files: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("persona dataset %s contains no personas", directory)
	}

	personas := make([]*Persona, 0, len(paths))
	ids := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		profile, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read persona profile %s: %w", path, err)
		}

		var record datasetPersona
		if err := yaml.Unmarshal(profile, &record); err != nil {
			return nil, fmt.Errorf("decode persona profile %s: %w", path, err)
		}

		if record.ID == "" || record.Source == "" || record.DisplayName == "" {
			return nil, fmt.Errorf("persona profile %s is missing an identity field", path)
		}

		if _, exists := ids[record.ID]; exists {
			return nil, fmt.Errorf("persona dataset contains duplicate persona ID %s", record.ID)
		}

		ids[record.ID] = struct{}{}
		personas = append(personas, &Persona{
			ID:          record.ID,
			Version:     record.Version,
			Source:      record.Source,
			DisplayName: record.DisplayName,
			Dimensions:  record.Dimensions,
			Profile:     string(profile),
		})
	}

	if datasetManifest.Count != len(personas) {
		return nil, fmt.Errorf("persona dataset manifest count %d does not match %d profiles", datasetManifest.Count, len(personas))
	}

	return &Repository{personas: personas}, nil
}

func (r *Repository) Count() int {
	return len(r.personas)
}

func (r *Repository) Query(query Query) []*Persona {
	result := make([]*Persona, 0, len(r.personas))
	for _, persona := range r.personas {
		if !matchesSource(persona, query.Sources) || !matchesDimensions(persona, query.DimensionFilters) {
			continue
		}

		result = append(result, persona)
	}

	return result
}

func matchesSource(persona *Persona, sources []string) bool {
	if len(sources) == 0 {
		return true
	}

	for _, source := range sources {
		if persona.Source == source {
			return true
		}
	}

	return false
}

func matchesDimensions(persona *Persona, filters map[string][]string) bool {
	for dimension, acceptedValues := range filters {
		value, exists := persona.Dimensions[dimension]
		if !exists || !contains(acceptedValues, value) {
			return false
		}
	}

	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
