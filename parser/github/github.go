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
	"github.com/urizennnn/autostandup-reposcanner/config"
	"golang.org/x/oauth2"
)

func CreateGithubClient(privateKey []byte, clientID string, installationID int64) *github.Client {
	fmt.Printf("Creating github client\n")
	fmt.Printf("Using installation ID: %d\n", installationID)
	appTokenSource, err := githubauth.NewApplicationTokenSource(clientID, privateKey)
	if err != nil {
		log.Fatalf("An Error occured when creating github client %v", err)
		return nil
	}
	installationTokenSource := githubauth.NewInstallationTokenSource(installationID, appTokenSource)
	httpClient := *oauth2.NewClient(context.Background(), installationTokenSource)
	client := github.NewClient(&httpClient)
	return client
}

func ListCommits(client *github.Client, owner, repo, branch, format string, since, until time.Time) (ai.StandupPayload, error) {
	fmt.Printf("Fetching commits")
	commits, _, err := client.Repositories.ListCommits(
		context.Background(), owner, repo, &github.CommitsListOptions{
			Since: since,
			Until: until,
			SHA:   branch,
		})
	if err != nil {
		log.Fatalf("Error fetching commits %s", err)
	}

	aiCommits := make([]ai.Commit, 0, len(commits))

	for _, c := range commits {
		if c == nil {
			continue
		}
		sha := c.GetSHA()

		name := c.GetCommit().GetAuthor().GetName()
		email := c.GetCommit().GetAuthor().GetEmail()
		if email == "" {
			email = c.GetCommit().GetCommitter().GetEmail()
		}
		if name == "" {
			name = c.GetCommit().GetCommitter().GetName()
		}

		login := c.GetAuthor().GetLogin()
		msg := c.GetCommit().GetMessage()

		files, adds, dels, err := GetCommitStats(client, owner, repo, sha)
		if err != nil {
			log.Printf("Error fetching commit stats for %s: %v", sha, err)
			continue
		}

		fmt.Printf("\nCommit: %s by %s <%s> github:%s files:%d +%d/-%d\n", sha, name, email, login, files, adds, dels)

		aiCommits = append(aiCommits, ai.Commit{
			SHA:         sha,
			AuthorName:  name,
			AuthorEmail: email,
			Message:     msg,
			Files:       files,
			Additions:   adds,
			Deletions:   dels,
		})
	}

	if len(aiCommits) == 0 {
		fmt.Println("\nNo commits to summarize")
		return ai.StandupPayload{}, nil
	}

	openaiAPIKey, err := config.FetchSecretByName("APP_OPENAI_API_KEY")
	if err != nil {
		log.Fatalf("An Error occured when fetching openai api key %v", err)
	}

	var res any
	switch strings.ToUpper(strings.ReplaceAll(format, "-", "_")) {
	case "TECHNICAL":
		res, err := ai.SummarizeTechinicalCommits(context.TODO(), openaiAPIKey, ai.SummarizeJob{
			Repo:        owner + "/" + repo,
			ProjectName: repo,
			Handle:      owner,
			Since:       since.UTC(),
			Until:       until.UTC(),
			Commits:     aiCommits,
		}, "technical")
		if err != nil {
			log.Printf("summarize error: %v", err)
		}
		return res, nil

	case "MILDLY_TECHNICAL":
		res, err := ai.SummarizeMildlyTechnicalCommits(context.TODO(), openaiAPIKey, ai.SummarizeJob{
			Repo:        owner + "/" + repo,
			ProjectName: repo,
			Handle:      owner,
			Since:       since.UTC(),
			Until:       until.UTC(),
			Commits:     aiCommits,
		}, "mildly_technical")
		if err != nil {
			log.Printf("summarize error: %v", err)
		}
		return res, nil

	case "LAYMAN":
		res, err := ai.SummarizeLaymanCommits(context.TODO(), openaiAPIKey, ai.SummarizeJob{
			Repo:        owner + "/" + repo,
			ProjectName: repo,
			Handle:      owner,
			Since:       since.UTC(),
			Until:       until.UTC(),
			Commits:     aiCommits,
		}, "layman")
		if err != nil {
			log.Printf("summarize error: %v", err)
		}
		return res, nil
	default:
		fmt.Printf("Unknown format: %s, defaulting to TECHNICAL\n", format)
	}
	fmt.Printf("\nSummary:\n%s\n", res)
	return ai.StandupPayload{}, nil
}

func GetCommitStats(client *github.Client, owner, repo, sha string) (files int, additions int, deletions int, err error) {
	commit, _, err := client.Repositories.GetCommit(context.Background(), owner, repo, sha, &github.ListOptions{})
	if err != nil {
		return 0, 0, 0, err
	}
	for _, f := range commit.Files {
		if f == nil {
			continue
		}
		files++
		additions += f.GetAdditions()
		deletions += f.GetDeletions()
	}
	return files, additions, deletions, nil
}
