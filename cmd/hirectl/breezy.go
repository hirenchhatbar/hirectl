package main

import (
	"context"
	"net/http"
)

func init() {
	integrations["breezy"] = &integration{fetch: fetchBreezy, hosts: []string{"*.breezy.hr"}}
}

// fetchBreezy reads a public Breezy HR job list: <company>.breezy.hr/json.
// shortcut: no descriptions (text is ignored), as the list has none and each needs its own request;
// discover tags techs from titles only. Fetch each job's page if title tags prove too thin.
func fetchBreezy(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	u, err := siteURL(company, "breezy.hr", "/json")
	if err != nil {
		return nil, err
	}
	var body []struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		Locations []struct {
			Name     string `json:"name"`
			IsRemote bool   `json:"is_remote"`
		} `json:"locations"`
		PublishedDate string `json:"published_date"`
	}
	if err := getJSON(ctx, client, u, &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body))
	for _, j := range body {
		var locs []string
		for _, l := range j.Locations {
			locs = append(locs, remote(l.IsRemote, l.Name))
		}
		jobs = append(jobs, Job{Title: j.Name, Location: locations(locs), URL: j.URL, Posted: posted(j.PublishedDate)})
	}
	return jobs, nil
}
