package models

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// ErrNoModel is returned when no listed model meets a Need.
var ErrNoModel = errors.New("xai: no model meets the requirements")

// Need describes the language model a caller wants. Zero fields do not filter.
type Need struct {
	MinContext      int64
	MaxPromptPrice  int64  // USD cents per 100M prompt tokens, the API's unit
	ReasoningEffort string // the model must accept this effort
	ImageInput      bool
	// Rank orders candidates; the first wins. The default is Newest.
	Rank func(a, b Candidate) int
}

// Candidate is a language model with the metadata Select filters on.
type Candidate struct {
	ListedModel
	ImageInput bool
}

// Rankings for Need.Rank.
var (
	Newest   = func(a, b Candidate) int { return cmp.Compare(b.Created, a.Created) }
	Cheapest = func(a, b Candidate) int { return cmp.Compare(a.PromptTextTokenPrice, b.PromptTextTokenPrice) }
	Largest  = func(a, b Candidate) int { return cmp.Compare(b.ContextLength, a.ContextLength) }
)

// Select returns the id of the best language model this key can call for need.
func (s *Service) Select(ctx context.Context, need Need) (string, error) {
	listed, err := s.All(ctx)
	if err != nil {
		return "", err
	}
	language, err := s.ListLanguage(ctx)
	if err != nil {
		return "", err
	}
	images := map[string]bool{}
	for _, m := range language.GetModels() {
		images[m.GetName()] = slices.Contains(m.GetInputModalities(), xaiapi.Modality_IMAGE)
	}
	var candidates []Candidate
	for _, m := range listed {
		if image, ok := images[m.ID]; ok {
			candidates = append(candidates, Candidate{ListedModel: m, ImageInput: image})
		}
	}
	return Choose(candidates, need)
}

// Choose applies need to candidates without calling the API.
func Choose(candidates []Candidate, need Need) (string, error) {
	fit := slices.DeleteFunc(slices.Clone(candidates), func(c Candidate) bool {
		return c.ContextLength < need.MinContext ||
			(need.MaxPromptPrice > 0 && c.PromptTextTokenPrice > need.MaxPromptPrice) ||
			(need.ImageInput && !c.ImageInput) ||
			(need.ReasoningEffort != "" && (c.Capabilities == nil || !slices.Contains(c.Capabilities.ReasoningEffort, need.ReasoningEffort)))
	})
	if len(fit) == 0 {
		return "", ErrNoModel
	}
	rank := need.Rank
	if rank == nil {
		rank = Newest
	}
	slices.SortStableFunc(fit, rank)
	return fit[0].ID, nil
}
