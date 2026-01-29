package github

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/jferrl/go-githubauth"
	"github.com/urizennnn/autostandup-reposcanner/ai"
	"github.com/urizennnn/autostandup-reposcanner/cache"
	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/ratelimit"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

func NewClient(cfg *config.Config, privateKey []byte, clientID string, installationID int64) (*Client, error) {
	limiter := ratelimit.New(cfg.GithubRateLimit, cfg.OpenaiRateLimit)

	c, err := cache.New(cfg.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("creating cache: %w", err)
	}

	ghClient, err := createGithubClient(privateKey, clientID, installationID, cfg.HTTPClientTimeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		gh:      ghClient,
		limiter: limiter,
		cache:   c,
		config:  cfg,
	}, nil
}

func createGithubClient(privateKey []byte, clientID string, installationID int64, timeout time.Duration) (*github.Client, error) {
	log.Printf("[INFO] creating github client for installation %d", installationID)

	appTokenSource, err := githubauth.NewApplicationTokenSource(clientID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}

	installationTokenSource := githubauth.NewInstallationTokenSource(installationID, appTokenSource)

	baseClient := oauth2.NewClient(context.Background(), installationTokenSource)
	baseClient.Timeout = timeout

	return github.NewClient(baseClient), nil
}

func (c *Client) ListCommits(
	ctx context.Context,
	owner string,
	repo string,
	branch string,
	format string,
	since time.Time,
	until time.Time,
) (ai.SummarizeResult, error) {
	log.Printf("[INFO] fetching commits %s/%s branch=%s", owner, repo, branch)

	commits, _, err := c.gh.Repositories.ListCommits(
		ctx,
		owner,
		repo,
		&github.CommitsListOptions{
			Since: since,
			Until: until,
			SHA:   branch,
		},
	)
	if err != nil {
		return ai.SummarizeResult{}, fmt.Errorf("fetching commits: %w", err)
	}

	if len(commits) == 0 {
		return ai.SummarizeResult{}, nil
	}

	results := make([]ai.Commit, len(commits))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.config.GithubConcurrency)

	for i, commit := range commits {
		if commit == nil {
			continue
		}

		i := i
		commit := commit

		g.Go(func() error {
			sha := commit.GetSHA()

			author := commit.GetCommit().GetAuthor()
			committer := commit.GetCommit().GetCommitter()

			name := author.GetName()
			email := author.GetEmail()

			if name == "" {
				name = committer.GetName()
			}
			if email == "" {
				email = committer.GetEmail()
			}

			files, adds, dels, err := c.getCommitStats(ctx, owner, repo, sha)
			if err != nil {
				log.Printf("[WARN] commit stats error %s: %v", sha, err)
				return nil
			}

			results[i] = ai.Commit{
				SHA:         sha,
				AuthorName:  name,
				AuthorEmail: email,
				Message:     commit.GetCommit().GetMessage(),
				Files:       files,
				Additions:   adds,
				Deletions:   dels,
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return ai.SummarizeResult{}, err
	}

	aiCommits := make([]ai.Commit, 0, len(results))
	for _, r := range results {
		if r.SHA != "" {
			aiCommits = append(aiCommits, r)
		}
	}

	openaiAPIKey, err := config.FetchSecretByName("APP_OPENAI_API_KEY")
	if err != nil {
		return ai.SummarizeResult{}, fmt.Errorf("fetching openai api key: %w", err)
	}

	job := ai.SummarizeJob{
		Repo:        owner + "/" + repo,
		ProjectName: repo,
		Handle:      owner,
		Window: ai.TimeWindow{
			Since: since.UTC(),
			Until: until.UTC(),
		},
		Commits: aiCommits,
	}

	formatType := normalizeFormat(format)

	return ai.Summarize(ctx, openaiAPIKey, job, formatType)
}

func normalizeFormat(format string) ai.StandupFormat {
	switch strings.ToUpper(strings.ReplaceAll(format, "-", "_")) {
	case "LAYMAN":
		return ai.FormatLayman
	case "MILDLY_TECHNICAL":
		return ai.FormatMildlyTechnical
	case "TECHNICAL":
		return ai.FormatTechnical
	default:
		return ai.FormatTechnical
	}
}

func (c *Client) getCommitStats(
	ctx context.Context,
	owner string,
	repo string,
	sha string,
) (int, int, int, error) {
	cacheKey := fmt.Sprintf("commit:%s:%s:%s", owner, repo, sha)

	if cached, ok := c.cache.Get(cacheKey); ok {
		stats := cached.(commitStats)
		return stats.Files, stats.Additions, stats.Deletions, nil
	}

	if err := c.limiter.WaitGithub(ctx); err != nil {
		return 0, 0, 0, err
	}

	commit, _, err := c.gh.Repositories.GetCommit(
		ctx,
		owner,
		repo,
		sha,
		&github.ListOptions{},
	)
	if err != nil {
		return 0, 0, 0, err
	}

	var stats commitStats
	for _, f := range commit.Files {
		if f == nil {
			continue
		}
		stats.Files++
		stats.Additions += f.GetAdditions()
		stats.Deletions += f.GetDeletions()
	}

	c.cache.Set(cacheKey, stats, time.Hour)

	return stats.Files, stats.Additions, stats.Deletions, nil
}
