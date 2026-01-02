package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

type Limiter struct {
	github *rate.Limiter
	openai *rate.Limiter
}

func New(githubReqPerMin, openaiReqPerMin int) *Limiter {
	return &Limiter{
		github: rate.NewLimiter(rate.Limit(float64(githubReqPerMin)/60.0), githubReqPerMin),
		openai: rate.NewLimiter(rate.Limit(float64(openaiReqPerMin)/60.0), openaiReqPerMin),
	}
}

func (l *Limiter) WaitGithub(ctx context.Context) error {
	return l.github.Wait(ctx)
}

func (l *Limiter) WaitOpenAI(ctx context.Context) error {
	return l.openai.Wait(ctx)
}
