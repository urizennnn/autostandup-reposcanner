package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func Summarize(ctx context.Context, apiKey string, job SummarizeJob, format StandupFormat) (SummarizeResult, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return SummarizeResult{}, err
	}

	tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:       "emit_structured_standup",
		Parameters: buildSchema(),
	})

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt(format)),
			openai.UserMessage(string(jobJSON)),
		},
		Tools: []openai.ChatCompletionToolUnionParam{tool},
	}

	baseCtx := context.Background()

	newCtx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
	defer cancel()

	resp, err := client.Chat.Completions.New(newCtx, params)
	if err != nil {
		return SummarizeResult{}, err
	}

	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return SummarizeResult{}, fmt.Errorf("no tool call returned")
	}

	var payload StandupPayload
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == "emit_structured_standup" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &payload); err != nil {
				return SummarizeResult{}, err
			}
			break
		}
	}

	if payload.Repo == "" {
		return SummarizeResult{}, fmt.Errorf("empty payload")
	}

	usage := UsageDetails{
		Model:            string(resp.Model),
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		EstimatedCost:    calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens),
	}

	log.Printf(
		"[INFO] standup generated repo=%s commits=%d contributors=%d format=%s",
		payload.Repo,
		len(job.Commits),
		len(payload.Contributors),
		format,
	)

	return SummarizeResult{
		Payload: payload,
		Usage:   usage,
	}, nil
}

func calculateCost(prompt, completion int64) float64 {
	return (float64(prompt)*2.5)/1_000_000 +
		(float64(completion)*10.0)/1_000_000
}

func systemPrompt(format StandupFormat) string {
	base := `You are AutoStandup.
Return ONE function call named "emit_structured_standup".
Return valid JSON matching the provided schema exactly.
Aggregate commits truthfully. Deduplicate similar work.
Convert timestamps to RFC3339.`

	switch format {
	case FormatTechnical:
		return base + `
Tone: technical.
Focus on architecture, refactors, performance, and code-level impact.`

	case FormatMildlyTechnical:
		return base + `
Tone: mildly technical.
Explain changes clearly without deep implementation detail.`

	case FormatLayman:
		return base + `
Tone: layman.
Use plain language. No engineering jargon.`

	default:
		return base
	}
}

func buildSchema() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"repo":        map[string]any{"type": "string"},
			"projectName": map[string]any{"type": "string"},
			"window": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"since": map[string]any{"type": "string", "format": "date-time"},
					"until": map[string]any{"type": "string", "format": "date-time"},
				},
				"required": []string{"since", "until"},
			},
			"summary": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":      map[string]any{"type": "string"},
					"highlights": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"keyChanges": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"impact":     map[string]any{"type": "string"},
					"nextFocus":  map[string]any{"type": "string"},
				},
				"required": []string{"title", "highlights", "keyChanges", "impact", "nextFocus"},
			},
			"metrics": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filesChanged": map[string]any{"type": "integer"},
					"additions":    map[string]any{"type": "integer"},
					"deletions":    map[string]any{"type": "integer"},
					"commits": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"filesChanged", "additions", "deletions"},
			},
			"contributors": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":    map[string]any{"type": "string"},
						"email":   map[string]any{"type": "string"},
						"commits": map[string]any{"type": "integer"},
					},
					"required": []string{"name", "commits"},
				},
			},
		},
		"required": []string{
			"repo",
			"projectName",
			"window",
			"summary",
			"metrics",
		},
	}
}
