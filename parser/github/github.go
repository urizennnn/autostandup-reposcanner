package github

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/jferrl/go-githubauth"
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

func ListCommits(client *github.Client) []string {
	var sha []string
	fmt.Printf("Fetching commits")
	commits, _, err := client.Repositories.ListCommits(
		context.Background(), "urizennnn", "autostandup-reposcanner", &github.CommitsListOptions{
			Since: time.Now().AddDate(0, 0, -1), // last 7 days
			Until: time.Now(),
		})
	if err != nil {
		log.Fatalf("Error fetching commits %s", err)
	}
	for _, commit := range commits {
		sha = append(sha, *commit.SHA)
	}
	return sha
}

func GetCommit(client *github.Client, sha string) *github.RepositoryCommit {
	commit, _, err := client.Repositories.GetCommit(context.Background(), "urizennnn", "autostandup-reposcanner", sha, &github.ListOptions{})
	if err != nil {
		log.Fatalf("Error fetching commit %s", err)
	}
	for _, f := range commit.Files {
		if f == nil {
			continue
		}
		// Most fields in go-github are pointers: *string, *int, etc.
		// Check for nil before deref, or use a tiny helper to default them.
		name := ""
		if f.Filename != nil {
			name = *f.Filename
		}

		adds, dels := 0, 0
		if f.Additions != nil {
			adds = *f.Additions
		}
		if f.Deletions != nil {
			dels = *f.Deletions
		}

		fmt.Printf("\nShowing Files")
		fmt.Printf("%s +%d/-%d %s\n", name, adds, dels, *f.Patch)
	}
	return nil
}
