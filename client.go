package ghcomment

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/go-github/v50/github"
	"github.com/shurcooL/githubv4"
)

type Client struct {
	// GitHub REST API.
	c *github.Client

	// GitHub GraphQL API client.
	// Used to hide old comments.
	//
	// TODO I've been looking at the GraphQL API and it seems it could
	// be possible to use that to retrieve the comments and changed
	// files, however, due to the time-limitation of the hackathon we
	// won't explore that option.
	g *githubv4.Client
}

func NewClient(ctx context.Context, c *http.Client) Client {
	return Client{
		c: github.NewClient(c),
		g: githubv4.NewClient(c),
	}
}

func (c Client) Comment(ctx context.Context, org, repo string, nr int, body string) error {
	_, _, err := c.c.Issues.CreateComment(ctx, org, repo, nr, &github.IssueComment{
		Body: github.String(body),
	})
	return err
}

func (c Client) HideCommentsMatching(ctx context.Context, org, repo string, nr int, re *regexp.Regexp) error {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		cs, res, err := c.c.Issues.ListComments(ctx, org, repo, nr, opts)
		if err != nil {
			return fmt.Errorf("retrieving PR comments: %w", err)
		}

		for _, cm := range cs {
			if !re.MatchString(cm.GetBody()) {
				continue
			}

			// hide comment
			var m struct {
				MinimizeComment struct {
					MinimizedComment struct {
						IsMinimized githubv4.Boolean
					}
				} `graphql:"minimizeComment(input: $input)"`
			}

			i := githubv4.MinimizeCommentInput{
				SubjectID:  cm.GetNodeID(),
				Classifier: githubv4.ReportedContentClassifiersOutdated,
			}

			if err := c.g.Mutate(ctx, &m, i, nil); err != nil {
				return fmt.Errorf("hiding comment %v: %w", cm.GetHTMLURL(), err)
			}
		}

		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}

	return nil
}
