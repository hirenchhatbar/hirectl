package main

import (
	"context"
	"html"
	"net/http"
	"net/url"
)

func init() {
	integrations["greenhouse"] = &integration{fetch: fetchGreenhouse, hosts: []string{"boards.greenhouse.io", "job-boards.greenhouse.io"}}
}

// fetchGreenhouse reads a public Greenhouse job board: https://developers.greenhouse.io/job-board.html
func fetchGreenhouse(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body struct {
		Jobs []struct {
			Title       string `json:"title"`
			AbsoluteURL string `json:"absolute_url"`
			Location    struct {
				Name string `json:"name"`
			} `json:"location"`
			Content        string `json:"content"` // entity-escaped HTML
			FirstPublished string `json:"first_published"`
		} `json:"jobs"`
	}
	u := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(company) + "/jobs"
	if text {
		u += "?content=true"
	}
	if err := getJSON(ctx, client, u, &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		jobs = append(jobs, Job{Title: j.Title, Location: j.Location.Name, URL: j.AbsoluteURL, Posted: posted(j.FirstPublished), text: html.UnescapeString(j.Content)})
	}
	return jobs, nil
}
