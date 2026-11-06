package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Job is one posting found on a board for a feed, or, in suggestions.json, for no feed.
type Job struct {
	Feed     string    `json:"feed,omitempty"` // feed ID
	Board    string    `json:"board"`          // e.g. greenhouse:stripe
	Title    string    `json:"title"`
	Location string    `json:"location,omitempty"`
	URL      string    `json:"url,omitempty"`
	Tech     []string  `json:"tech,omitempty"`  // tech names in title or text, set by discover
	Posted   time.Time `json:"posted,omitzero"` // when first published, zero when the site gives none
	Score    int       `json:"score,omitempty"` // how much it is about its feed's skills, set by run
	Level    string    `json:"level,omitempty"` // a levels name, years of experience asked, set by run; blank when unknown
	text     string    // description, used for matching only
}

// postedLayouts are the formats job sites give a posting date in.
var postedLayouts = []string{time.RFC3339, "2006-01-02 15:04:05 MST", time.DateOnly}

// posted parses a job site's posting date in UTC; a blank or unknown one is the zero time,
// so a bad date never fails a board.
func posted(s string) time.Time {
	for _, l := range postedLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// integration is one job site.
type integration struct {
	// fetch returns every open job on one company's board; it fills Title, Location, URL,
	// and text only when text is true, as descriptions make responses much larger.
	fetch func(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error)
	// hosts are the board page hosts `hirectl discover` searches: "<host>" for https://<host>/<company>/...,
	// "*.<domain>" for https://<company>.<domain>/...
	hosts []string
	// company, when set, returns the company of a crawled page on hosts, or "" for none, in place of the above.
	company func(host, path string) string
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

// requestTimeout bounds one board request; Lever at times takes over 30s to answer a page.
const requestTimeout = 2 * time.Minute

// errNotFound marks a 404, such as a board that does not exist.
var errNotFound = errors.New("not found")

// statusError is a response other than 200, 206 or 404.
type statusError struct {
	req  string // e.g. "GET https://..."
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s: %d %s", e.req, e.code, http.StatusText(e.code))
}

// get GETs url, only the byte range rng ("0-99") when set, and returns the body on 200 or 206.
func get(ctx context.Context, client *http.Client, url, rng string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if rng != "" {
		req.Header.Set("Range", "bytes="+rng)
	}
	return send(client, req)
}

// send sends req and returns the body on 200 or 206.
func send(client *http.Client, req *http.Request) (io.ReadCloser, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return resp.Body, nil
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL, errNotFound)
	}
	resp.Body.Close()
	return nil, &statusError{req.Method + " " + req.URL.String(), resp.StatusCode}
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

// postJSON POSTs in as JSON to url and decodes the JSON answer into out.
func postJSON(ctx context.Context, client *http.Client, url string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	body, err := send(client, req)
	if err != nil {
		return err
	}
	defer body.Close()
	return json.NewDecoder(body).Decode(out)
}

var labelRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// siteURL returns https://<company>.<domain><path> for a site giving each company a subdomain.
// A company that is no host label, such as a Workday board, does not exist there.
func siteURL(company, domain, path string) (string, error) {
	if !labelRe.MatchString(company) {
		return "", fmt.Errorf("%q: %w", company, errNotFound)
	}
	return "https://" + company + "." + domain + path, nil
}

// remote prefixes loc with "Remote" when is: feeds match location words, so remote jobs must say so.
func remote(is bool, loc string) string {
	switch {
	case !is:
		return loc
	case loc == "":
		return "Remote"
	}
	return "Remote, " + loc
}

// locations joins a job's non-blank locations.
func locations(locs []string) string {
	return strings.Join(slices.DeleteFunc(slices.Clone(locs), func(l string) bool { return strings.TrimSpace(l) == "" }), "; ")
}
