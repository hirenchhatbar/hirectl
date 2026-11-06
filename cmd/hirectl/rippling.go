package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	integrations["rippling"] = &integration{fetch: fetchRippling, hosts: []string{"ats.rippling.com"}}
}

// fetchRippling reads a public Rippling job board: api.rippling.com/platform/api/ats/v1/board/<company>/jobs.
// shortcut: no descriptions (text is ignored), as the list has none and each needs its own request;
// discover tags techs from titles only. Fetch .../jobs/<uuid> per job if title tags prove too thin.
func fetchRippling(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body []struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		WorkLocation struct {
			Label string `json:"label"`
		} `json:"workLocation"`
	}
	if err := getJSON(ctx, client, "https://api.rippling.com/platform/api/ats/v1/board/"+url.PathEscape(company)+"/jobs", &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body))
	for _, j := range body {
		jobs = append(jobs, Job{Title: strings.TrimSpace(j.Name), Location: j.WorkLocation.Label, URL: j.URL})
	}
	return jobs, nil
}
