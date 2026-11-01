package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/gofri/go-github-pagination/githubpagination"
	"github.com/google/go-github/v76/github"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

// Config struct holds runtime configuration required for the application
type Config struct {
	MetricsPath              string   `envconfig:"METRICS_PATH" required:"false" default:"/metrics"`
	ListenPort               string   `envconfig:"LISTEN_PORT" required:"false" default:"9171"`
	LogLevel                 string   `envconfig:"LOG_LEVEL" required:"false" default:"INFO"`
	ApiUrl                   *url.URL `envconfig:"API_URL" required:"false" default:"https://api.github.com"`
	Repositories             []string `envconfig:"REPOS" required:"false"`
	Organisations            []string `envconfig:"ORGS" required:"false"`
	Users                    []string `envconfig:"USERS" required:"false"`
	GitHubResultsPerPage     int      `envconfig:"GITHUB_RESULTS_PER_PAGE" required:"false" default:"100"`
	GithubToken              string   `envconfig:"GITHUB_TOKEN" required:"false"`
	GithubTokenFile          string   `envconfig:"GITHUB_TOKEN_FILE" required:"false"`
	GitHubApp                bool     `envconfig:"GITHUB_APP" required:"false" default:"false"`
	GitHubRateLimitEnabled   bool     `envconfig:"GITHUB_RATE_LIMIT_ENABLED" required:"false" default:"true"`
	GitHubRateLimit          float64  `envconfig:"GITHUB_RATE_LIMIT" required:"false" default:"0"`
	FetchRepoReleasesEnabled bool     `envconfig:"FETCH_REPO_RELEASES_ENABLED" required:"false" default:"true"`
	FetchOrgsConcurrency     int      `envconfig:"FETCH_ORGS_CONCURRENCY" required:"false" default:"1"`
	FetchOrgReposConcurrency int      `envconfig:"FETCH_ORG_REPOS_CONCURRENCY" required:"false" default:"1"`
	FetchReposConcurrency    int      `envconfig:"FETCH_REPOS_CONCURRENCY" required:"false" default:"1"`
	FetchUsersConcurrency    int      `envconfig:"FETCH_USERS_CONCURRENCY" required:"false" default:"1"`
	*GitHubAppConfig         `ignored:"true"`
}

type GitHubAppConfig struct {
	GitHubAppKeyPath        string `envconfig:"GITHUB_APP_KEY_PATH" required:"false" default:""`
	GitHubAppId             int64  `envconfig:"GITHUB_APP_ID" required:"false" default:""`
	GitHubAppInstallationId int64  `envconfig:"GITHUB_APP_INSTALLATION_ID" required:"false" default:""`
}

// Init populates the Config struct based on environmental runtime configuration
func Init() (*Config, error) {

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("processing envconfig: %v", err)
	}

	// Parse and set log level
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("parsing log level: %v", err)
	}
	log.SetLevel(level)

	// Trim whitespace and drop empty entries from repositories, organisations, and users
	cfg.Repositories = cleanList(cfg.Repositories)
	cfg.Organisations = cleanList(cfg.Organisations)
	cfg.Users = cleanList(cfg.Users)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Process GitHub App config if enabled
	if cfg.GitHubApp {
		var appCfg GitHubAppConfig
		if err := envconfig.Process("", &appCfg); err != nil {
			return nil, fmt.Errorf("processing GitHub App envconfig: %v", err)
		}
		cfg.GitHubAppConfig = &appCfg
	}

	// Read token from file if not set directly
	if cfg.GithubToken == "" && cfg.GithubTokenFile != "" {
		tokenBytes, err := os.ReadFile(cfg.GithubTokenFile)
		if err != nil {
			return nil, fmt.Errorf("reading GitHub token from file: %v", err)
		}
		cfg.GithubToken = strings.TrimSpace(string(tokenBytes))
	}

	return &cfg, nil
}

func (c *Config) GetClient() (*github.Client, error) {
	transport := http.DefaultTransport

	// Add custom transport for GitHub App authentication if enabled
	if c.GitHubApp {
		itr, err := ghinstallation.NewKeyFromFile(
			transport,
			c.GitHubAppId,
			c.GitHubAppInstallationId,
			c.GitHubAppKeyPath,
		)
		if err != nil {
			return nil, fmt.Errorf("creating GitHub App installation transport: %v", err)
		}

		if c.ApiUrl != nil && c.ApiUrl.String() != "https://api.github.com" {
			itr.BaseURL = c.ApiUrl.String()
		}

		transport = itr
	}

	paginator := githubpagination.NewClient(transport,
		githubpagination.WithPerPage(c.GitHubResultsPerPage),
	)

	client := github.NewClient(paginator)

	if c.ApiUrl != nil && c.ApiUrl.String() != "https://api.github.com" {
		// Don't need to validate error as it's checked in envconfig
		client, _ = client.WithEnterpriseURLs(c.ApiUrl.String(), c.ApiUrl.String())
	}

	if c.GithubToken != "" {
		client = client.WithAuthToken(c.GithubToken)
	}

	return client, nil
}

// validate checks for values that would otherwise only fail once scraping starts.
func (c *Config) validate() error {
	for _, repo := range c.Repositories {
		parts := strings.Split(repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid repository %q in REPOS: expected owner/repo", repo)
		}
	}

	concurrency := []struct {
		name  string
		value int
	}{
		{"FETCH_ORGS_CONCURRENCY", c.FetchOrgsConcurrency},
		{"FETCH_ORG_REPOS_CONCURRENCY", c.FetchOrgReposConcurrency},
		{"FETCH_REPOS_CONCURRENCY", c.FetchReposConcurrency},
		{"FETCH_USERS_CONCURRENCY", c.FetchUsersConcurrency},
	}
	for _, setting := range concurrency {
		if setting.value < 1 {
			return fmt.Errorf("invalid %s %d: must be at least 1", setting.name, setting.value)
		}
	}

	return nil
}

// cleanList trims whitespace from each entry and removes any that are empty,
// such as those left by a trailing comma.
func cleanList(input []string) []string {
	result := make([]string, 0, len(input))
	for _, e := range input {
		if e = strings.TrimSpace(e); e != "" {
			result = append(result, e)
		}
	}
	return result
}
