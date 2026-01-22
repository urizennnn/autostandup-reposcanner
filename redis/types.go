package redis

import "time"

type contextKey string

type QueueMessage struct {
	Owner          string    `json:"owner"`
	IsTestStandup  bool      `json:"isTestStandup"`
	Repo           string    `json:"repo"`
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	InstallationID int64     `json:"installation_id"`
	Branch         string    `json:"branch"`
	Format         string    `json:"format"`
}
