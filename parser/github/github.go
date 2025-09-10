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

func ListCommits(client *github.Client) {
	fmt.Printf("Fetching commits")
	commits, _, err := client.Repositories.ListCommits(
		context.Background(), "urizennnn", "autostandup-reposcanner", &github.CommitsListOptions{
			Since: time.Now().AddDate(0, 0, -1), // last 7 days
			Until: time.Now(),
		})
	if err != nil {
		log.Fatalf("Error fetching commits %s", err)
	}
	for _, c := range commits {
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

		fmt.Printf("\nCommit: %s by %s <%s> github:%s\n", sha, name, email, login)
		GetCommit(client, sha)
	}
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
