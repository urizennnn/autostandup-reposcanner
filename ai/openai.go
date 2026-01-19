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

type Commit struct {
	SHA         string `json:"sha"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Message     string `json:"message"`
	Files       int    `json:"files"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

type SummarizeJob struct {
	Repo        string    `json:"repo"`
	ProjectName string    `json:"projectName"`
	Handle      string    `json:"handle"`
	Since       time.Time `json:"since"`
	Until       time.Time `json:"until"`
	Commits     []Commit  `json:"commits"`
}

type Contributor struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Commits int    `json:"commits"`
}

type FilesChanged struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type TechnicalLevel struct {
	Header       string       `json:"header"`
	WhatWorkedOn []string     `json:"whatWorkedOn,omitempty"`
	FilesChanged FilesChanged `json:"filesChanged"`
	Commits      []string     `json:"commits,omitempty"`
}

type SummaryLevel struct {
	Header       string   `json:"header"`
	WhatWorkedOn []string `json:"whatWorkedOn,omitempty"`
	Impact       string   `json:"impact"`
	Focus        string   `json:"focus"`
}

type StandupPayload struct {
	Repo   string `json:"repo"`
	Title  string `json:"title"`
	Window struct {
		Since string `json:"since"`
		Until string `json:"until"`
	} `json:"window"`
	Technical       TechnicalLevel `json:"technical"`
	MildlyTechnical SummaryLevel   `json:"mildlyTechnical"`
	Layman          SummaryLevel   `json:"layman"`
	Contributors    []Contributor  `json:"contributors,omitempty"`
}

type FormatType string

const (
	FormatTechnical       FormatType = "technical"
	FormatMildlyTechnical FormatType = "mildly_technical"
	FormatLayman          FormatType = "layman"
)

type UsageDetails struct {
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	EstimatedCost    float64 `json:"estimatedCost"`
}

type SummarizeResult struct {
	Payload StandupPayload `json:"payload"`
	Details UsageDetails   `json:"details"`
}

func Summarize(ctx context.Context, apiKey string, job SummarizeJob, format FormatType) (SummarizeResult, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return SummarizeResult{}, fmt.Errorf("marshal job: %w", err)
	}

	tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "emit_structured_standup",
		Description: openai.String("Return the final standup payload in the exact structure the app expects."),
		Parameters:  buildSchema(format),
	})

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Seed:  openai.Int(0),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(getSystemPrompt(format)),
			openai.UserMessage(fmt.Sprintf(`{"instruction":"Summarize commits into the exact structure","payload":%s}`, string(jobJSON))),
		},
		Tools: []openai.ChatCompletionToolUnionParam{tool},
	}

	chatCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := client.Chat.Completions.New(chatCtx, params)
	if err != nil {
		log.Printf("[ERROR] chat completion error: %v", err)
		return SummarizeResult{}, err
	}
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return SummarizeResult{}, fmt.Errorf("model did not return tool call")
	}
	var out StandupPayload
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == "emit_structured_standup" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &out); err != nil {
				return SummarizeResult{}, fmt.Errorf("bad tool args: %w", err)
			}
			break
		}
	}
	if out.Repo == "" {
		return SummarizeResult{}, fmt.Errorf("empty payload")
	}

	log.Printf("[INFO] summary generated repo=%s since=%s until=%s contributors=%d format=%s",
		out.Repo, out.Window.Since, out.Window.Until, len(out.Contributors), format)

	pruneOutput(&out, format)

	details := UsageDetails{
		Model:            string(resp.Model),
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		EstimatedCost:    calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens),
	}

	return SummarizeResult{Payload: out, Details: details}, nil
}

func calculateCost(promptTokens, completionTokens int64) float64 {
	// GPT-4o pricing: $2.50/1M input, $10/1M output
	return (float64(promptTokens) * 2.5 / 1_000_000) + (float64(completionTokens) * 10.0 / 1_000_000)
}

func getSystemPrompt(format FormatType) string {
	switch format {
	case FormatTechnical:
		return `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema.
Shape content technical level:
- technical: header, whatWorkedOn bullets, filesChanged {files, additions, deletions}, commits[] (short, conventional commit style).
	technical should be on the same understanding level as a software engineer it should contain the changes made and how it affected the codebase in regards to improvement and efficieny.
Convert time stamps into human readable dates.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName and separate the commits summary for the different contributors". Include in the result a title for the standup.`
	case FormatMildlyTechnical:
		return `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema.
Shape content mildly-technical level only:
	Convert time stamps into human readable dates.
- mildlyTechnical: header, whatWorkedOn bullets, impact, focus.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName". Include in the result a title for the standup.`
	case FormatLayman:
		return `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema.
Shape content layman level only:
	Convert time stamps into human readable dates.

- layman: header, whatWorkedOn bullets (plain language), impact, focus.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName". Include in the result a title for the standup.`
	default:
		return ""
	}
}

func buildSchema(format FormatType) openai.FunctionParameters {
	baseProps := map[string]any{
		"repo": map[string]any{"type": "string"},
		"window": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"since": map[string]any{"type": "string"},
				"until": map[string]any{"type": "string"},
			},
			"required": []string{"since", "until"},
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
		"title": map[string]any{"type": "string"},
	}

	var required []string
	switch format {
	case FormatTechnical:
		baseProps["technical"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":  map[string]any{"type": "string"},
				"header": map[string]any{"type": "string"},
				"whatWorkedOn": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"filesChanged": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"files":     map[string]any{"type": "integer"},
						"additions": map[string]any{"type": "integer"},
						"deletions": map[string]any{"type": "integer"},
					},
					"required": []string{"files", "additions", "deletions"},
				},
				"commits": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"changes": map[string]any{
				"type": "object",
				"items": map[string]any{
					"Code lines Added":        map[string]any{"type": "integer"},
					"Code lines Deleted":      map[string]any{"type": "integer"},
					"Code characters added":   map[string]any{"type": "integer"},
					"Code characters deleted": map[string]any{"type": "integer"},
				},
			},
			"required": []string{"header", "filesChanged", "title"},
		}
		required = []string{"repo", "window", "technical", "title"}

	case FormatMildlyTechnical:
		baseProps["mildlyTechnical"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":  map[string]any{"type": "string"},
				"header": map[string]any{"type": "string"},
				"whatWorkedOn": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"impact": map[string]any{"type": "string"},
				"focus":  map[string]any{"type": "string"},
			},
			"required": []string{"header", "impact", "focus", "title"},
		}
		baseProps["changes"] = map[string]any{
			"type": "object",
			"items": map[string]any{
				"Code lines Added":        map[string]any{"type": "integer"},
				"Code lines Deleted":      map[string]any{"type": "integer"},
				"Code characters added":   map[string]any{"type": "integer"},
				"Code characters deleted": map[string]any{"type": "integer"},
			},
		}
		required = []string{"repo", "window", "mildlyTechnical", "title"}

	case FormatLayman:
		baseProps["layman"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":  map[string]any{"type": "string"},
				"header": map[string]any{"type": "string"},
				"whatWorkedOn": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"impact": map[string]any{"type": "string"},
				"focus":  map[string]any{"type": "string"},
			},
			"required": []string{"header", "impact", "focus", "title"},
		}
		baseProps["changes"] = map[string]any{
			"type": "object",
			"items": map[string]any{
				"Code lines Added":        map[string]any{"type": "integer"},
				"Code lines Deleted":      map[string]any{"type": "integer"},
				"Code characters added":   map[string]any{"type": "integer"},
				"Code characters deleted": map[string]any{"type": "integer"},
			},
		}
		required = []string{"repo", "window", "layman", "title"}
	}

	return openai.FunctionParameters{
		"type":       "object",
		"properties": baseProps,
		"required":   required,
	}
}

func pruneOutput(out *StandupPayload, format FormatType) {
	title := out.Title // Preserve the title before pruning
	switch format {
	case FormatTechnical:
		out.MildlyTechnical = SummaryLevel{}
		out.Layman = SummaryLevel{}
		out.Technical.WhatWorkedOn = pruneEmpty(out.Technical.WhatWorkedOn)
		out.Technical.Commits = pruneEmpty(out.Technical.Commits)
	case FormatMildlyTechnical:
		out.Technical = TechnicalLevel{}
		out.Layman = SummaryLevel{}
		out.MildlyTechnical.WhatWorkedOn = pruneEmpty(out.MildlyTechnical.WhatWorkedOn)
	case FormatLayman:
		out.Technical = TechnicalLevel{}
		out.MildlyTechnical = SummaryLevel{}
		out.Layman.WhatWorkedOn = pruneEmpty(out.Layman.WhatWorkedOn)
	}
	out.Title = title // Restore the title after pruning
}

func pruneEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
