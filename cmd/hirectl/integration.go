package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Job is one posting found on a board for a feed, or, in suggestions.json, for no feed.
type Job struct {
	Feed     string   `json:"feed,omitempty"` // feed ID
	Board    string   `json:"board"`          // e.g. greenhouse:stripe
	Title    string   `json:"title"`
	Location string   `json:"location,omitempty"`
	URL      string   `json:"url,omitempty"`
	Tech     []string `json:"tech,omitempty"` // tech names in title or text, set by discover
	text     string   // description, used for matching only
}

// integration is one job site.
type integration struct {
	// fetch returns every open job on one company's board; it fills Title, Location, URL,
	// and text only when text is true, as descriptions make responses much larger.
	fetch func(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error)
	hosts []string // board page hosts, as in https://<host>/<company>/..., searched by `hirectl discover`
}

// integrations maps a board prefix, as in "greenhouse:stripe", to its integration.
// Each job site lives in its own file and registers itself here from init.
var integrations = map[string]*integration{}

func integrationNames() []string { return slices.Sorted(maps.Keys(integrations)) }

// sourcesUsage is the --sources flag help shared by run and discover.
func sourcesUsage() string {
	return `comma-separated integrations to use, any of: ` + strings.Join(integrationNames(), ", ") + `
e.g. --sources greenhouse, or --sources greenhouse,lever (default: all)`
}

// checkSources reports the first name that is not an integration.
func checkSources(names []string) error {
	for _, s := range names {
		if integrations[s] == nil {
			return fmt.Errorf("unknown source %q, have %s", s, strings.Join(integrationNames(), ", "))
		}
	}
	return nil
}

// splitBoard splits "integration:company"; ok is false when either part is blank.
func splitBoard(b string) (name, company string, ok bool) {
	name, company, _ = strings.Cut(b, ":")
	name, company = strings.TrimSpace(name), strings.TrimSpace(company)
	return name, company, name != "" && company != ""
}

// pacer fetches boards, waiting at least delay between requests to the same integration.
// It is not safe for concurrent use.
type pacer struct {
	client *http.Client
	delay  time.Duration
	last   map[string]time.Time // integration name to its last request
}

func newPacer(client *http.Client, delay time.Duration) *pacer {
	return &pacer{client: client, delay: delay, last: map[string]time.Time{}}
}

// fetch returns the jobs on board, with descriptions when text is true.
func (p *pacer) fetch(ctx context.Context, board string, text bool) ([]Job, error) {
	name, company, _ := splitBoard(board)
	in := integrations[name]
	if in == nil {
		return nil, errors.New("unknown integration")
	}
	select { // on cancel, fetch fails with the context error
	case <-ctx.Done():
	case <-time.After(time.Until(p.last[name].Add(p.delay))):
	}
	defer func() { p.last[name] = time.Now() }()
	return in.fetch(ctx, p.client, company, text)
}

// errNotFound marks a 404, such as a board that does not exist.
var errNotFound = errors.New("not found")

// get GETs url, only the byte range rng ("0-99") when set, and returns the body on 200 or 206.
func get(ctx context.Context, client *http.Client, url, rng string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if rng != "" {
		req.Header.Set("Range", "bytes="+rng)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return resp.Body, nil
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %w", url, errNotFound)
	}
	resp.Body.Close()
	return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
}

// getJSON GETs url and decodes its JSON body into v.
func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	body, err := get(ctx, client, url, "")
	if err != nil {
		return err
	}
	defer body.Close()
	return json.NewDecoder(body).Decode(v)
}
