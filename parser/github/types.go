package github

import (
	"github.com/google/go-github/v74/github"
	"github.com/urizennnn/autostandup-reposcanner/cache"
	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/ratelimit"
)

type Client struct {
	gh      *github.Client
	limiter *ratelimit.Limiter
	cache   *cache.Cache
	config  *config.Config
}

type commitStats struct {
	Files     int
	Additions int
	Deletions int
}
