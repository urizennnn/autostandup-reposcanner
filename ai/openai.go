package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/sirupsen/logrus"
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
	Window struct {
		Since string `json:"since"`
		Until string `json:"until"`
	} `json:"window"`
	Technical       TechnicalLevel `json:"technical"`
	MildlyTechnical SummaryLevel   `json:"mildlyTechnical"`
	Layman          SummaryLevel   `json:"layman"`
	Contributors    []Contributor  `json:"contributors,omitempty"`
}

func SummarizeTechinicalCommits(ctx context.Context, apiKey string, job SummarizeJob, format string) (StandupPayload, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	sys := `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema. 
Shape content technical level: 
- technical: header, whatWorkedOn bullets, filesChanged {files, additions, deletions}, commits[] (short, conventional commit style).
	technical should be on the same understanding level as a software engineer it should contain the changes made and how it affected the codebase in regards to improvement and efficieny.
Convert time stamps into human readable dates.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName and separate the commits summary for the different contributors".`

	jobJSON, _ := json.Marshal(job)

	tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "emit_structured_standup",
		Description: openai.String("Return the final standup payload in the exact structure the app expects."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{"type": "string"},
				"window": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"since": map[string]any{"type": "string"},
						"until": map[string]any{"type": "string"},
					},
					"required": []string{"since", "until"},
				},
				"technical": map[string]any{
					"type": "object",
					"properties": map[string]any{
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
					// Allow omitting empty arrays by not requiring them
					"required": []string{"header", "filesChanged"},
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
			"required": []string{"repo", "window", "technical"},
		},
	})

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Seed:  openai.Int(0),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(fmt.Sprintf(`{"instruction":"Summarize commits into the exact structure","payload":%s}`, string(jobJSON))),
		},
		Tools: []openai.ChatCompletionToolUnionParam{tool},
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return StandupPayload{}, err
	}
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return StandupPayload{}, fmt.Errorf("model did not return tool call")
	}

	var out StandupPayload
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == "emit_structured_standup" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &out); err != nil {
				return StandupPayload{}, fmt.Errorf("bad tool args: %w", err)
			}
			break
		}
	}
	if out.Repo == "" {
		return StandupPayload{}, fmt.Errorf("empty payload")
	}

	logrus.WithFields(logrus.Fields{
		"repo":    out.Repo,
		"since":   out.Window.Since,
		"until":   out.Window.Until,
		"authors": len(out.Contributors),
	}).Info("summarizer: structured standup ready")
	// Prune empty arrays to keep structure clean
	out.Technical.WhatWorkedOn = pruneEmpty(out.Technical.WhatWorkedOn)
	out.Technical.Commits = pruneEmpty(out.Technical.Commits)

	return out, nil
}

func SummarizeMildlyTechnicalCommits(ctx context.Context, apiKey string, job SummarizeJob, format string) (StandupPayload, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	sys := `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema.
Shape content mildly-technical level only:
	Convert time stamps into human readable dates.
- mildlyTechnical: header, whatWorkedOn bullets, impact, focus.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName".`

	jobJSON, _ := json.Marshal(job)

	tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "emit_structured_standup",
		Description: openai.String("Return the final standup payload in the exact structure the app expects."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{"type": "string"},
				"window": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"since": map[string]any{"type": "string"},
						"until": map[string]any{"type": "string"},
					},
					"required": []string{"since", "until"},
				},
				"mildlyTechnical": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"header": map[string]any{"type": "string"},
						"whatWorkedOn": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"impact": map[string]any{"type": "string"},
						"focus":  map[string]any{"type": "string"},
					},
					"required": []string{"header", "impact", "focus"},
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
				"changes": map[string]any{
					"type": "object",
					"items": map[string]any{
						"Code lines Added":        map[string]any{"type": "integer"},
						"Code lines Deleted":      map[string]any{"type": "integer"},
						"Code characters added":   map[string]any{"type": "integer"},
						"Code characters deleted": map[string]any{"type": "integer"},
					},
				},
			},
			"required": []string{"repo", "window", "mildlyTechnical"},
		},
	})

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Seed:  openai.Int(0),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(fmt.Sprintf(`{"instruction":"Summarize commits into the exact structure","payload":%s}`, string(jobJSON))),
		},
		Tools: []openai.ChatCompletionToolUnionParam{tool},
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return StandupPayload{}, err
	}
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return StandupPayload{}, fmt.Errorf("model did not return tool call")
	}

	var out StandupPayload
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == "emit_structured_standup" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &out); err != nil {
				return StandupPayload{}, fmt.Errorf("bad tool args: %w", err)
			}
			break
		}
	}
	if out.Repo == "" {
		return StandupPayload{}, fmt.Errorf("empty payload")
	}

	logrus.WithFields(logrus.Fields{
		"repo":    out.Repo,
		"since":   out.Window.Since,
		"until":   out.Window.Until,
		"authors": len(out.Contributors),
	}).Info("summarizer: structured standup ready")

	out.MildlyTechnical.WhatWorkedOn = pruneEmpty(out.MildlyTechnical.WhatWorkedOn)

	return out, nil
}

func SummarizeLaymanCommits(ctx context.Context, apiKey string, job SummarizeJob, format string) (StandupPayload, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	sys := `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema.
Shape content layman level only:
	Convert time stamps into human readable dates.

- layman: header, whatWorkedOn bullets (plain language), impact, focus.
Keep it concise, truthful, de-duplicate similar commits, and aggregate. Use the provided handle and projectName in headers like: "📊 **Daily Standup for @handle** – ProjectName".`

	jobJSON, _ := json.Marshal(job)

	tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "emit_structured_standup",
		Description: openai.String("Return the final standup payload in the exact structure the app expects."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{"type": "string"},
				"window": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"since": map[string]any{"type": "string"},
						"until": map[string]any{"type": "string"},
					},
					"required": []string{"since", "until"},
				},
				"layman": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"header": map[string]any{"type": "string"},
						"whatWorkedOn": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"impact": map[string]any{"type": "string"},
						"focus":  map[string]any{"type": "string"},
					},
					"required": []string{"header", "impact", "focus"},
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
				"changes": map[string]any{
					"type": "object",
					"items": map[string]any{
						"Code lines Added":        map[string]any{"type": "integer"},
						"Code lines Deleted":      map[string]any{"type": "integer"},
						"Code characters added":   map[string]any{"type": "integer"},
						"Code characters deleted": map[string]any{"type": "integer"},
					},
				},
			},
			"required": []string{"repo", "window", "layman"},
		},
	})

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Seed:  openai.Int(0),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(fmt.Sprintf(`{"instruction":"Summarize commits into the exact structure","payload":%s}`, string(jobJSON))),
		},
		Tools: []openai.ChatCompletionToolUnionParam{tool},
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return StandupPayload{}, err
	}
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return StandupPayload{}, fmt.Errorf("model did not return tool call")
	}

	var out StandupPayload
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name == "emit_structured_standup" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &out); err != nil {
				return StandupPayload{}, fmt.Errorf("bad tool args: %w", err)
			}
			break
		}
	}
	if out.Repo == "" {
		return StandupPayload{}, fmt.Errorf("empty payload")
	}

	logrus.WithFields(logrus.Fields{
		"repo":    out.Repo,
		"since":   out.Window.Since,
		"until":   out.Window.Until,
		"authors": len(out.Contributors),
	}).Info("summarizer: structured standup ready")

	out.Layman.WhatWorkedOn = pruneEmpty(out.Layman.WhatWorkedOn)

	return out, nil
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
