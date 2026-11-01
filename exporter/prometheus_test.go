package exporter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githubexporter/github-exporter/config"
	"github.com/google/go-github/v76/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempRSAKey generates a 2048-bit RSA private key and writes it as a
// PKCS#1 PEM file to a temp path. The file is removed automatically when the
// test finishes.
func writeTempRSAKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "test-github-app-key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	require.NoError(t, os.WriteFile(path, keyPEM, 0o600))

	return path
}

func newTestExporter(cfg config.Config) *Exporter {
	return &Exporter{
		Client: github.NewClient(nil),
		Config: cfg,
	}
}

// newCollectExporter returns an exporter for cfg that sends requests to server.
func newCollectExporter(t *testing.T, server *httptest.Server, cfg config.Config) *Exporter {
	t.Helper()

	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)

	cfg.GitHubRateLimitEnabled = true
	cfg.FetchRepoReleasesEnabled = true
	cfg.FetchReposConcurrency = max(cfg.FetchReposConcurrency, 1)
	cfg.FetchOrgsConcurrency = max(cfg.FetchOrgsConcurrency, 1)
	cfg.FetchOrgReposConcurrency = max(cfg.FetchOrgReposConcurrency, 1)
	cfg.FetchUsersConcurrency = max(cfg.FetchUsersConcurrency, 1)

	e := newTestExporter(cfg)
	e.APIMetrics = AddMetrics(&cfg)
	e.Client.BaseURL = baseURL
	return e
}

// collectCounts runs Collect and returns how many repositories and rate limit
// resources were exported.
func collectCounts(e *Exporter) (repos, rates int) {
	ch := make(chan prometheus.Metric, 10000)
	e.Collect(ch)
	close(ch)

	for m := range ch {
		desc := m.Desc().String()
		if strings.Contains(desc, "github_repo_stars") {
			repos++
		}
		if strings.Contains(desc, "github_rate_remaining") {
			rates++
		}
	}
	return repos, rates
}

// newRepoServer mocks the GitHub API, with each organisation and user owning
// reposPerOwner repositories. Requests to a path in failures return that status.
func newRepoServer(t *testing.T, reposPerOwner int, failures map[string]int) *httptest.Server {
	t.Helper()

	repoJSON := func(owner, name string) string {
		return fmt.Sprintf(`{"name":%q,"owner":{"login":%q}}`, name, owner)
	}
	listRepos := func(w http.ResponseWriter, r *http.Request) {
		repos := make([]string, reposPerOwner)
		for i := range repos {
			repos[i] = repoJSON(r.PathValue("owner"), fmt.Sprintf("repo%d", i))
		}
		_, _ = fmt.Fprintf(w, "[%s]", strings.Join(repos, ","))
	}
	emptyList := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "[]")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/{owner}/repos", listRepos)
	mux.HandleFunc("GET /users/{owner}/repos", listRepos)
	mux.HandleFunc("GET /repos/{owner}/{repo}/releases", emptyList)
	mux.HandleFunc("GET /repos/{owner}/{repo}/pulls", emptyList)
	mux.HandleFunc("GET /repos/{owner}/{repo}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, repoJSON(r.PathValue("owner"), r.PathValue("repo")))
	})
	mux.HandleFunc("GET /rate_limit", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"resources":{"core":{"limit":5000,"remaining":0,"reset":1}}}`)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status, ok := failures[r.URL.Path]; ok {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, `{"message":"request failed"}`)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestCollectHandlesFailures checks that a failed request only removes the
// metrics that depend on it. A panic in Collect would crash the exporter.
func TestCollectHandlesFailures(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		status    int
		wantRepos int
		wantRates int
	}{
		{"repository forbidden", "/repos/myOrg/repo0", http.StatusForbidden, 0, 1},
		{"user repositories forbidden", "/users/myOrg/repos", http.StatusForbidden, 1, 1},
		{"organisation repositories forbidden", "/orgs/myOrg/repos", http.StatusForbidden, 1, 1},
		{"releases forbidden", "/repos/myOrg/repo0/releases", http.StatusForbidden, 0, 1},
		{"pull requests forbidden", "/repos/myOrg/repo0/pulls", http.StatusForbidden, 0, 1},
		{"rate limit forbidden", "/rate_limit", http.StatusForbidden, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newRepoServer(t, 1, map[string]int{tt.path: tt.status})
			e := newCollectExporter(t, server, config.Config{
				Repositories:  []string{"myOrg/repo0"},
				Organisations: []string{"myOrg"},
				Users:         []string{"myOrg"},
			})

			var repos, rates int
			assert.NotPanics(t, func() { repos, rates = collectCounts(e) })
			assert.Equal(t, tt.wantRepos, repos)
			assert.Equal(t, tt.wantRates, rates)
		})
	}

	t.Run("rate limit without resources", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `{}`)
		}))
		defer server.Close()
		e := newCollectExporter(t, server, config.Config{})

		var rates int
		assert.NotPanics(t, func() { _, rates = collectCounts(e) })
		assert.Zero(t, rates)
	})
}

func TestCollectSkipsFailures(t *testing.T) {
	const blocked = http.StatusUnavailableForLegalReasons

	t.Run("repository in REPOS", func(t *testing.T) {
		server := newRepoServer(t, 0, map[string]int{"/repos/o/bad/pulls": blocked})
		e := newCollectExporter(t, server, config.Config{Repositories: []string{"o/one", "o/bad", "o/two"}})

		repos, rates := collectCounts(e)
		assert.Equal(t, 2, repos)
		assert.Equal(t, 1, rates)
	})

	t.Run("repository in organisation", func(t *testing.T) {
		server := newRepoServer(t, 3, map[string]int{"/repos/o/repo1/pulls": blocked})
		e := newCollectExporter(t, server, config.Config{Organisations: []string{"o"}})

		repos, rates := collectCounts(e)
		assert.Equal(t, 2, repos)
		assert.Equal(t, 1, rates)
	})

	t.Run("repository owned by user", func(t *testing.T) {
		server := newRepoServer(t, 3, map[string]int{"/repos/u/repo1/pulls": blocked})
		e := newCollectExporter(t, server, config.Config{Users: []string{"u"}})

		repos, rates := collectCounts(e)
		assert.Equal(t, 2, repos)
		assert.Equal(t, 1, rates)
	})

	t.Run("organisation and user", func(t *testing.T) {
		server := newRepoServer(t, 2, map[string]int{
			"/orgs/badOrg/repos":   http.StatusNotFound,
			"/users/badUser/repos": http.StatusNotFound,
		})
		e := newCollectExporter(t, server, config.Config{
			Organisations: []string{"badOrg", "goodOrg"},
			Users:         []string{"badUser", "goodUser"},
		})

		repos, rates := collectCounts(e)
		assert.Equal(t, 4, repos)
		assert.Equal(t, 1, rates)
	})

	t.Run("invalid repository format", func(t *testing.T) {
		server := newRepoServer(t, 0, nil)
		e := newCollectExporter(t, server, config.Config{Repositories: []string{"invalid", "o/one"}})

		repos, _ := collectCounts(e)
		assert.Equal(t, 1, repos)
	})
}

// TestCollectConcurrent checks no repositories are lost when fetched
// concurrently. Run with -race to also detect unsynchronised access.
func TestCollectConcurrent(t *testing.T) {
	server := newRepoServer(t, 50, nil)

	repositories := make([]string, 50)
	for i := range repositories {
		repositories[i] = fmt.Sprintf("r/repo%d", i)
	}

	e := newCollectExporter(t, server, config.Config{
		Repositories:             repositories,
		Organisations:            []string{"o1", "o2"},
		Users:                    []string{"u1", "u2"},
		FetchReposConcurrency:    8,
		FetchOrgsConcurrency:     2,
		FetchOrgReposConcurrency: 8,
		FetchUsersConcurrency:    2,
	})

	repos, rates := collectCounts(e)
	assert.Equal(t, 250, repos)
	assert.Equal(t, 1, rates)
}

func TestRateLimitRefresh(t *testing.T) {
	// Shared rate-limit slices used across sub-tests.
	aboveThreshold := []RateLimit{{Resource: "core", Remaining: 200, Limit: 5000}}
	atThreshold := []RateLimit{{Resource: "core", Remaining: 100, Limit: 5000}}
	belowThreshold := []RateLimit{{Resource: "core", Remaining: 50, Limit: 5000}}
	noCoreEntry := []RateLimit{{Resource: "search", Remaining: 5, Limit: 30}}

	baseCfg := config.Config{
		GitHubResultsPerPage:     100,
		FetchReposConcurrency:    1,
		FetchOrgsConcurrency:     1,
		FetchOrgReposConcurrency: 1,
		FetchUsersConcurrency:    1,
	}

	t.Run("no-op when GitHubRateLimit is zero", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 0
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&atThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when GitHubRateLimit is negative", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = -1
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&atThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when rates is nil", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(nil)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when core remaining is above threshold", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100 // remaining=200 > threshold=100
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&aboveThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when rates has no core entry", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&noCoreEntry)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when GitHubApp is false and core is at threshold", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100 // remaining=100 <= threshold=100
		cfg.GitHubApp = false
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&atThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when GitHubApp is false and core is below threshold", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100 // remaining=50 < threshold=100
		cfg.GitHubApp = false
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&belowThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("no-op when GitHubApp is true but GetClient fails", func(t *testing.T) {
		cfg := baseCfg
		cfg.GitHubRateLimit = 100
		cfg.GitHubApp = true
		cfg.GitHubAppConfig = &config.GitHubAppConfig{
			GitHubAppKeyPath:        "/nonexistent/key.pem",
			GitHubAppId:             1,
			GitHubAppInstallationId: 1,
		}
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&atThreshold)

		assert.Same(t, original, e.Client)
	})

	t.Run("replaces client when GitHubApp is true and core is at threshold", func(t *testing.T) {
		keyPath := writeTempRSAKey(t)
		cfg := baseCfg
		cfg.GitHubRateLimit = 100 // remaining=100 <= threshold=100
		cfg.GitHubApp = true
		cfg.GitHubAppConfig = &config.GitHubAppConfig{
			GitHubAppKeyPath:        keyPath,
			GitHubAppId:             1,
			GitHubAppInstallationId: 1,
		}
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&atThreshold)

		assert.NotSame(t, original, e.Client, "expected client to be replaced after rate limit refresh")
	})

	t.Run("replaces client when GitHubApp is true and core is below threshold", func(t *testing.T) {
		keyPath := writeTempRSAKey(t)
		cfg := baseCfg
		cfg.GitHubRateLimit = 100 // remaining=50 < threshold=100
		cfg.GitHubApp = true
		cfg.GitHubAppConfig = &config.GitHubAppConfig{
			GitHubAppKeyPath:        keyPath,
			GitHubAppId:             1,
			GitHubAppInstallationId: 1,
		}
		e := newTestExporter(cfg)
		original := e.Client

		e.rateLimitRefresh(&belowThreshold)

		assert.NotSame(t, original, e.Client, "expected client to be replaced after rate limit refresh")
	})
}
