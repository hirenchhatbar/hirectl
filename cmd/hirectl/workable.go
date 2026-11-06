package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	integrations["workable"] = &integration{fetch: fetchWorkable, hosts: []string{"apply.workable.com"}}
}

// fetchWorkable reads a public Workable job widget: apply.workable.com/api/v1/widget/accounts/<company>.
func fetchWorkable(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body struct {
		Jobs []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			City          string `json:"city"`
			State         string `json:"state"`
			Country       string `json:"country"`
			Telecommuting bool   `json:"telecommuting"`
			Description   string `json:"description"` // HTML
			PublishedOn   string `json:"published_on"`
		} `json:"jobs"`
	}
	u := "https://apply.workable.com/api/v1/widget/accounts/" + url.PathEscape(company)
	if text {
		u += "?details=true"
	}
	if err := getJSON(ctx, client, u, &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		var loc []string
		for _, s := range []string{j.City, j.State, j.Country} {
			if s != "" {
				loc = append(loc, s)
			}
		}
		jobs = append(jobs, Job{Title: j.Title, Location: remote(j.Telecommuting, strings.Join(loc, ", ")), URL: j.URL, Posted: posted(j.PublishedOn), text: j.Description})
	}
	return jobs, nil
}
