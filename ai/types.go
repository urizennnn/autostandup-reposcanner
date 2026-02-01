package ai

import (
	"encoding/json"
	"time"
)

type Commit struct {
	SHA         string `json:"sha"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail,omitempty"`
	Files       int    `json:"files"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

type TimeWindow struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

type SummarizeJob struct {
	Repo        string     `json:"repo"`
	ProjectName string     `json:"projectName"`
	Handle      string     `json:"handle"`
	Window      TimeWindow `json:"window"`
	Commits     []Commit   `json:"commits"`
}

type ActivityMetrics struct {
	FilesChanged int      `json:"filesChanged"`
	Additions    int      `json:"additions"`
	Deletions    int      `json:"deletions"`
	Commits      []string `json:"commits,omitempty"`
}

type Contributor struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Commits int    `json:"commits"`
}

type TechnicalSummary struct {
	Overview         string   `json:"overview"`
	Accomplishments  []string `json:"accomplishments"`
	TechnicalDetails []string `json:"technicalDetails"`
	CodeImpact       string   `json:"codeImpact"`
}

type MildlyTechnicalSummary struct {
	Overview        string   `json:"overview"`
	Accomplishments []string `json:"accomplishments"`
	Changes         []string `json:"changes"`
	Impact          string   `json:"impact"`
}

type LaymanSummary struct {
	Overview        string   `json:"overview"`
	Accomplishments []string `json:"accomplishments"`
	Achievements    []string `json:"achievements"`
	BusinessValue   string   `json:"businessValue"`
}

type StandupPayload struct {
	Repo         string          `json:"repo"`
	ProjectName  string          `json:"projectName"`
	Window       TimeWindow      `json:"window"`
	Summary      json.RawMessage `json:"summary"`
	Metrics      ActivityMetrics `json:"metrics"`
	Contributors []Contributor   `json:"contributors,omitempty"`
}

type StandupFormat string

const (
	FormatTechnical       StandupFormat = "technical"
	FormatMildlyTechnical StandupFormat = "mildly_technical"
	FormatLayman          StandupFormat = "layman"
)

type RenderedStandup struct {
	Format StandupFormat `json:"format"`
	Text   string        `json:"text"`
	Blocks any           `json:"blocks,omitempty"`
}

type UsageDetails struct {
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	EstimatedCost    float64 `json:"estimatedCost"`
}

type SummarizeResult struct {
	Payload StandupPayload `json:"payload"`
	Usage   UsageDetails   `json:"usage"`
}
