package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"app/internal/clients"
	"app/internal/repositories/audience"
)

type AudiencePipeline struct {
	audiences *audience.Repository
}

type completedJobResult struct {
	Personas            []json.RawMessage          `json:"personas"`
	PersonaResearch     []json.RawMessage          `json:"persona_research"`
	Top3                []completedJobRanking      `json:"top_3"`
	HypothesesByPersona map[string]json.RawMessage `json:"hypotheses_by_persona"`
}

type completedJobRanking struct {
	PersonaID      string          `json:"persona_id"`
	Metrics        json.RawMessage `json:"metrics"`
	SelectionOrder int             `json:"selection_order"`
	FinalScore     float64         `json:"final_score"`
}

type generatorPersonaReference struct {
	PersonaID string `json:"persona_id"`
}

type generatedAudience struct {
	personaID  string
	persona    json.RawMessage
	research   json.RawMessage
	metrics    json.RawMessage
	hypotheses json.RawMessage
	rank       int
	score      float64
}

func NewAudiencePipeline(audiences *audience.Repository) *AudiencePipeline {
	return &AudiencePipeline{audiences: audiences}
}

func (AudiencePipeline) Kind() string {
	return "audiences"
}

func (p *AudiencePipeline) Persist(ctx context.Context, job *clients.CompletedJob) error {
	generated, err := parseAudienceResult(job.Result)
	if err != nil {
		return err
	}

	inputs := make([]audience.GeneratedInput, 0, len(generated))
	for _, item := range generated {
		inputs = append(inputs, audience.GeneratedInput{
			PersonaID:  item.personaID,
			Persona:    item.persona,
			Research:   item.research,
			Metrics:    item.metrics,
			Hypotheses: item.hypotheses,
			Rank:       item.rank,
			Score:      item.score,
		})
	}

	if err := p.audiences.PersistGenerated(ctx, job.ID, job.ProductID, inputs); err != nil {
		return fmt.Errorf("persist generated audiences: %w", err)
	}

	return nil
}

func parseAudienceResult(result json.RawMessage) ([]generatedAudience, error) {
	var parsed completedJobResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("decode completed generator result: %w", err)
	}

	personas, err := indexGeneratorValues(parsed.Personas)
	if err != nil {
		return nil, fmt.Errorf("index completed generator personas: %w", err)
	}

	research, err := indexGeneratorValues(parsed.PersonaResearch)
	if err != nil {
		return nil, fmt.Errorf("index completed generator research: %w", err)
	}

	audiences := make([]generatedAudience, 0, 3)
	for _, ranking := range parsed.Top3 {
		if ranking.SelectionOrder < 1 || ranking.SelectionOrder > 3 {
			return nil, fmt.Errorf("completed generator result has invalid selection order for persona %q", ranking.PersonaID)
		}

		persona, ok := personas[ranking.PersonaID]
		if !ok {
			return nil, fmt.Errorf("completed generator result is missing persona %q", ranking.PersonaID)
		}

		personaResearch, ok := research[ranking.PersonaID]
		if !ok {
			return nil, fmt.Errorf("completed generator result is missing research for persona %q", ranking.PersonaID)
		}

		hypotheses, ok := parsed.HypothesesByPersona[ranking.PersonaID]
		if !ok {
			return nil, fmt.Errorf("completed generator result is missing hypotheses for persona %q", ranking.PersonaID)
		}

		if len(ranking.Metrics) == 0 {
			return nil, fmt.Errorf("completed generator result is missing metrics for persona %q", ranking.PersonaID)
		}

		audiences = append(audiences, generatedAudience{
			personaID:  ranking.PersonaID,
			persona:    persona,
			research:   personaResearch,
			metrics:    ranking.Metrics,
			hypotheses: hypotheses,
			rank:       ranking.SelectionOrder,
			score:      ranking.FinalScore,
		})
	}

	sort.Slice(audiences, func(i, j int) bool { return audiences[i].rank < audiences[j].rank })
	for index, generated := range audiences {
		if generated.rank != index+1 {
			return nil, errors.New("completed generator result contains duplicate or missing audience ranks")
		}
	}

	return audiences, nil
}

func indexGeneratorValues(values []json.RawMessage) (map[string]json.RawMessage, error) {
	indexed := make(map[string]json.RawMessage, len(values))
	for _, value := range values {
		var reference generatorPersonaReference
		if err := json.Unmarshal(value, &reference); err != nil {
			return nil, fmt.Errorf("decode generator value: %w", err)
		}

		if reference.PersonaID == "" {
			return nil, errors.New("generator value did not contain persona_id")
		}

		indexed[reference.PersonaID] = value
	}

	return indexed, nil
}
