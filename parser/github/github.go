package github

import (
	"context"
	"fmt"
	"log"

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
	commits, githubResp, err := client.Repositories.ListCommits(
		context.Background(), "urizennnn", "autostandup-reposcanner", &github.CommitsListOptions{})
	if err != nil {
		log.Fatalf("Error fetching commits %s", err)
	}
	fmt.Printf(`%s %v`, commits, githubResp)
}
