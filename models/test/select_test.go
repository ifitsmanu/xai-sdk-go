package models_test

import (
	"errors"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/models"
)

// Shapes and values taken from the live /v1/models response on 2026-11-01.
var live = []models.Candidate{
	{ListedModel: models.ListedModel{ID: "grok-4.3", Created: 1776384000, ContextLength: 1_000_000, PromptTextTokenPrice: 12500,
		Capabilities: &models.Capabilities{ReasoningEffort: []string{"none", "low", "medium", "high", "xhigh"}}}, ImageInput: true},
	{ListedModel: models.ListedModel{ID: "grok-4.7", Created: 1788307200, ContextLength: 500_000, PromptTextTokenPrice: 20000,
		Capabilities: &models.Capabilities{ReasoningEffort: []string{"low", "medium", "high"}}}, ImageInput: true},
	{ListedModel: models.ListedModel{ID: "grok-build-0.1", Created: 1776297600, ContextLength: 256_000, PromptTextTokenPrice: 10000}},
}

func TestChoose(t *testing.T) {
	cases := []struct {
		name string
		need models.Need
		want string
	}{
		{"newest by default", models.Need{}, "grok-4.7"},
		{"context rules out newer models", models.Need{MinContext: 800_000}, "grok-4.3"},
		{"price cap", models.Need{MaxPromptPrice: 15000}, "grok-4.3"},
		{"reasoning effort must be supported", models.Need{ReasoningEffort: "xhigh"}, "grok-4.3"},
		{"image input", models.Need{ImageInput: true, Rank: models.Cheapest}, "grok-4.3"},
		{"custom ranking", models.Need{Rank: models.Cheapest}, "grok-build-0.1"},
	}
	for _, c := range cases {
		got, err := models.Choose(live, c.need)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
	if _, err := models.Choose(live, models.Need{MinContext: 2_000_000}); !errors.Is(err, models.ErrNoModel) {
		t.Errorf("impossible need: err = %v", err)
	}
}
