package exporter

import (
	"context"
	"github.com/google/go-github/v71/github"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strings"
)

// Describe - loops through the API metrics and passes them to prometheus.Describe
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {

	for _, m := range e.APIMetrics {
		ch <- m
	}

}

// Collect function, called on by Prometheus Client library
// This function is called when a scrape is performed on the /metrics endpoint
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	client := github.NewClient(nil)
	ctx := context.Background()

	log.Info("collecting metrics")
	var data []*Datum

	repoMetrics, err := e.getRepoMetrics(ctx)
	if err != nil {
		log.Errorf("Error fetching repository metrics: %v", err)
		return
	}

	data = append(data, repoMetrics...)

	// TODO - get all rate limits, not just core
	rates, _, err := client.RateLimit.Get(ctx)
	if err != nil {
		log.Errorf("Error fetching rate limits: %v", err)
		return
	}

	r := &RateLimits{
		Limit:     float64(rates.Core.Limit),
		Remaining: float64(rates.Core.Remaining),
		Reset:     float64(rates.Core.Reset.Unix()),
	}

	// Set prometheus gauge metrics using the data gathered
	err = e.processMetrics(data, r, ch)
}

// getOrganisationMetrics fetches metrics for the configured organisations
func (e *Exporter) getOrganisationMetrics(ctx context.Context) ([]*Datum, error) {
	var data []*Datum
	for _, o := range e.Config.Organisations {
		repos, _, err := e.Client.Repositories.ListByOrg(ctx, o, nil)
		if err != nil {
			log.Errorf("Error fetching organisation repositories: %v", err)
			continue
		}
		for _, repo := range repos {
			d, err := e.parseRepo(ctx, *repo)
			if err != nil {
				log.Errorf("Error parsing organisation data: %v", err)
				continue
			}
			data = append(data, d)
		}
	}

	return data, nil
}

// getUserMetrics fetches metrics for the configured users
func (e *Exporter) getUserMetrics(ctx context.Context) ([]*Datum, error) {
	var data []*Datum
	for _, u := range e.Config.Users {
		repos, _, err := e.Client.Repositories.ListByUser(ctx, u, nil)
		if err != nil {
			log.Errorf("Error fetching user data: %v", err)
			continue
		}

		for _, repo := range repos {
			d, err := e.parseRepo(ctx, *repo)
			if err != nil {
				log.Errorf("Error parsing user data: %v", err)
				continue
			}
			data = append(data, d)
		}
	}

	return data, nil
}

// getRepoMetrics fetches metrics for the configured repositories
func (e *Exporter) getRepoMetrics(ctx context.Context) ([]*Datum, error) {
	var data []*Datum
	for _, m := range e.Config.Repositories {
		// Split the repository string into owner and name
		parts := strings.Split(m, "/")
		if len(parts) != 2 {
			log.Errorf("Invalid repository format: %s", m)
			continue
		}

		repo, _, err := e.Client.Repositories.Get(ctx, parts[0], parts[1])
		if err != nil {
			log.Errorf("Error fetching repository data: %v", err)
			continue
		}

		d, err := e.parseRepo(ctx, *repo)
		if err != nil {
			log.Errorf("Error parsing repository data: %v", err)
			continue
		}

		data = append(data, d)
	}

	return data, nil
}

func (e *Exporter) parseRepo(ctx context.Context, repo github.Repository) (*Datum, error) {
	repoOwner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()

	rel, _, err := e.Client.Repositories.ListReleases(ctx, repoOwner, repoName, nil)
	if err != nil {
		log.Errorf("Error fetching releases: %v", err)
		return nil, err
	}

	var releases []Release
	for _, release := range rel {
		var assets []Asset
		for _, asset := range release.Assets {
			a := Asset{
				Name: asset.GetName(),
			}
			assets = append(assets, a)
		}

		r := Release{
			Name:   release.GetName(),
			Tag:    release.GetTagName(),
			Assets: assets,
		}
		releases = append(releases, r)
	}

	pullRequests, _, err := e.Client.PullRequests.List(ctx, repoOwner, repoName, nil)
	if err != nil {
		log.Errorf("Error fetching pull requests: %v", err)
		return nil, err
	}
	var pulls []Pull
	for _, pr := range pullRequests {
		p := Pull{
			Url: pr.GetURL(),
			User: User{
				Login: pr.GetUser().GetLogin(),
			},
		}
		pulls = append(pulls, p)
	}

	d := &Datum{
		Name: repo.GetName(),
		Owner: User{
			Login: repo.GetOwner().GetLogin(),
		},
		License: License{
			Key: repo.GetLicense().GetKey(),
		},
		Language:   repo.GetLanguage(),
		Archived:   repo.GetArchived(),
		Private:    repo.GetPrivate(),
		Fork:       repo.GetFork(),
		Forks:      float64(repo.GetForksCount()),
		Stars:      float64(repo.GetStargazersCount()),
		OpenIssues: float64(repo.GetOpenIssuesCount()),
		Watchers:   float64(repo.GetSubscribersCount()),
		Size:       float64(repo.GetSize()),
		Releases:   releases,
		Pulls:      pulls,
	}

	return d, nil
}
