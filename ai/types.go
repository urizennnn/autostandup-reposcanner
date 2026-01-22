package ai

import "time"

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
