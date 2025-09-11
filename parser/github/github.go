package github

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/jferrl/go-githubauth"
	"github.com/urizennnn/autostandup-reposcanner/ai"
	"github.com/urizennnn/autostandup-reposcanner/config"
	"golang.org/x/oauth2"
)

func CreateGithubClient(privateKey []byte, clientID string, installationID int64) *github.Client {
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

func ListCommits(client *github.Client) {
	owner := "urizennnn"
	repo := "autostandup-reposcanner"
	since := time.Now().AddDate(0, 0, -1)
	until := time.Now()

	fmt.Printf("Fetching commits")
	commits, _, err := client.Repositories.ListCommits(
		context.Background(), owner, repo, &github.CommitsListOptions{
			Since: since,
			Until: until,
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
		return
	}

	openaiAPIKey, err := config.FetchSecretByName("APP_OPENAI_API_KEY")
	if err != nil {
		log.Fatalf("An Error occured when fetching openai api key %v", err)
	}

	_, err = ai.SummarizeCommits(context.TODO(), openaiAPIKey, ai.SummarizeJob{
		Repo:        owner + "/" + repo,
		ProjectName: repo,
		Handle:      owner,
		Since:       since.UTC(),
		Until:       until.UTC(),
		Commits:     aiCommits,
	})
	if err != nil {
		log.Printf("summarize error: %v", err)
	}
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
