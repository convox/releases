package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/convox/praxis/api"
	"github.com/convox/praxis/cache"
	"github.com/pkg/errors"
)

func main() {
	server := api.New("releases", "releases.convox")

	server.Route("GET", "/", root)
	server.Route("GET", "/releases/{channel}", releases)
	server.Route("GET", "/releases/{channel}/next", next)

	if err := server.Listen("tcp", ":3000"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func root(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	return c.RenderOK()
}

func releases(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	rs, err := githubReleases()
	if err != nil {
		return errors.WithStack(err)
	}

	return c.RenderJSON(rs)
}

func next(w http.ResponseWriter, r *http.Request, c *api.Context) error {
	rs, err := githubReleases()
	if err != nil {
		return errors.WithStack(err)
	}

	if len(rs) < 1 {
		return fmt.Errorf("could not list releases")
	}

	return c.RenderJSON(rs[0])
}

func githubReleases() ([]string, error) {
	if v, ok := cache.Get("releases", nil).([]string); ok {
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
		Name string
	}

	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, errors.WithStack(err)
	}

	rs := []string{}

	for _, r := range releases {
		rs = append(rs, r.Name)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(rs)))

	if err := cache.Set("releases", nil, rs, 2*time.Minute); err != nil {
		return nil, errors.WithStack(err)
	}

	return rs, nil
}
