package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/convox/praxis/api"
	"github.com/convox/praxis/cache"
	"github.com/pkg/errors"
	"github.com/segmentio/analytics-go"
)

func main() {
	server := api.New("releases", "releases.convox")
	server.Use(segment())

	server.Route("GET", "/", root)
	server.Route("GET", "/releases/{channel}", releases)
	server.Route("GET", "/releases/{channel}/next", next)

	if err := server.Listen("tcp", ":3000"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func segment() api.Middleware {
	// The cx client should send a user agent like:
	// convox/$VERSION ($GOOS/$GOARCH) ($USERID)
	agentRE := regexp.MustCompile(`convox/.* \(.*/.*\) \((.*)\)`)

	client := analytics.New("TnKNy0zogmD7nNqqU6gAiH1wu226VZe9")
	client.Verbose = false // set to true for debugging

	return func(fn api.HandlerFunc) api.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, c *api.Context) error {
			// only report requests with a user id
			if m := agentRE.FindStringSubmatch(r.Header.Get("User-Agent")); m != nil {
				client.Page(&analytics.Page{
					Name:   r.URL.Path,
					UserId: m[1],
					Traits: map[string]interface{}{
						"path": r.URL.Path,
						"url":  fmt.Sprintf("https://%s%s", os.Getenv("HOST"), r.URL.Path),
					},
				})
			}

			return fn(w, r, c)
		}
	}
}

func root(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	return c.RenderOK()
}

func releases(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	channel := c.Var("channel")

	rs, err := githubReleases(channel)
	if err != nil {
		return errors.WithStack(err)
	}

	return c.RenderJSON(rs)
}

func next(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	channel := c.Var("channel")

	rs, err := githubReleases(channel)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(rs) < 1 {
		return fmt.Errorf("no releases for channel: %s", channel)
	}

	return c.RenderJSON(rs[0])
}

func githubReleases(channel string) ([]string, error) {
	if v, ok := cache.Get("releases", channel).([]string); ok {
		return v, nil
	}

	res, err := http.Get("https://api.github.com/repos/convox/praxis/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		Name       string
		Prerelease bool
	}

	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, errors.WithStack(err)
	}

	rs := []string{}

	fmt.Printf("channel = %+v\n", channel)

	for _, r := range releases {
		switch channel {
		case "edge":
			rs = append(rs, r.Name)
		case "stable":
			if !r.Prerelease {
				rs = append(rs, r.Name)
			}
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(rs)))

	if err := cache.Set("releases", channel, rs, 2*time.Minute); err != nil {
		return nil, errors.WithStack(err)
	}

	return rs, nil
}
