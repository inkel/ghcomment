package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v50/github"
	"github.com/gregjones/httpcache"
	"github.com/inkel/ghcomment"
	"golang.org/x/oauth2"
)

func main() {
	var cfg config

	flag.StringVar(&cfg.Owner, "owner", "", "repository owner")
	flag.StringVar(&cfg.Repo, "repo", "", "repository name")
	flag.IntVar(&cfg.Number, "nr", 0, "issue/pull request number")
	flag.StringVar(&cfg.Token, "token", "", "authentication token")
	flag.StringVar(&cfg.Body, "body", "", "comment body; prefix with @ for a file path")
	flag.StringVar(&cfg.HideRegexp, "hide-regexp", "", "regular expression to match comments to hide")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := realMain(ctx, cfg)
	cancel()

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func realMain(ctx context.Context, cfg config) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	httpClient, err := oauthClient(ctx, cfg)
	if err != nil {
		return err
	}

	c := ghcomment.NewClient(ctx, httpClient)

	body := cfg.Body

	if body[0] == '@' { // read from file
		b, err := os.ReadFile(body[1:])
		if err != nil {
			return err
		}
		body = string(b)
	}

	if err := c.Comment(ctx, cfg.Owner, cfg.Repo, cfg.Number, body); err != nil {
		return err
	}

	if cfg.HideRegexp != "" {
		re, err := regexp.Compile(cfg.HideRegexp)
		if err != nil {
			return err
		}

		if err := c.HideCommentsMatching(ctx, cfg.Owner, cfg.Repo, cfg.Number, re); err != nil {
			return err
		}
	}

	return nil
}

type config struct {
	Owner, Repo string
	Number      int

	Token             string
	AppID             int64
	AppPrivateKey     string
	AppInstallationID int64

	HideRegexp string

	Body string
}

func (c config) validate() error {
	if c.Owner == "" || c.Repo == "" {
		return errors.New("missing owner/repository")
	}
	if c.Number == 0 {
		return errors.New("invalid issue/PR number")
	}
	if (c.Token != "" && c.AppID > 0) || (c.Token == "" && c.AppID == 0) {
		return errors.New("invalid access configuration")
	}
	if c.AppID > 0 && (c.AppPrivateKey == "" || c.AppInstallationID == 0) {
		return errors.New("invalid app installation configuration")
	}
	if c.Body == "" {
		return errors.New("missing comment body")
	}
	return nil
}

func oauthClient(ctx context.Context, cfg config) (*http.Client, error) {
	var token = cfg.Token

	if cfg.AppID > 0 {
		t, err := ghinstallation.NewAppsTransport(
			httpcache.NewMemoryCacheTransport(),
			cfg.AppID,
			[]byte(cfg.AppPrivateKey),
		)
		if err != nil {
			return nil, fmt.Errorf("creating GitHub application installation transport: %w", err)
		}

		c := github.NewClient(&http.Client{Transport: t})

		tok, res, err := c.Apps.CreateInstallationToken(ctx, cfg.AppInstallationID, nil)
		if err != nil {
			return nil, fmt.Errorf("getting GitHub application installation token: %w", err)
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("unexpected status code from GitHub: %s", res.Status)
		}

		token = tok.GetToken()
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return oauth2.NewClient(ctx, ts), nil
}
