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
	WhatWorkedOn []string     `json:"whatWorkedOn"`
	FilesChanged FilesChanged `json:"filesChanged"`
	Commits      []string     `json:"commits"`
}

type SummaryLevel struct {
	Header       string   `json:"header"`
	WhatWorkedOn []string `json:"whatWorkedOn"`
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
	Contributors    []Contributor  `json:"contributors"`
}

func SummarizeCommits(ctx context.Context, apiKey string, job SummarizeJob) (StandupPayload, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	sys := `You are AutoStandup's summarizer. Output ONE function call "emit_structured_standup" with JSON that matches the provided schema. 
Shape content to three levels: 
- technical: header, whatWorkedOn bullets, filesChanged {files, additions, deletions}, commits[] (short, conventional commit style).
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
					"required": []string{"header", "whatWorkedOn", "filesChanged", "commits"},
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
					"required": []string{"header", "whatWorkedOn", "impact", "focus"},
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
					"required": []string{"header", "whatWorkedOn", "impact", "focus"},
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
			"required": []string{"repo", "window", "technical", "mildlyTechnical", "layman", "contributors"},
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
	fmt.Printf("Summarizer output: %+v\n", out)

	return out, nil
}
