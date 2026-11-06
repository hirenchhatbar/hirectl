package main

import (
	"context"
	"net/http"
	"net/url"
)

func init() {
	integrations["gem"] = &integration{fetch: fetchGem, hosts: []string{"jobs.gem.com"}}
}

// fetchGem reads a public Gem job board: api.gem.com/job_board/v0/<company>/job_posts/.
// The API always sends descriptions, so text only decides whether to keep them.
func fetchGem(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body []struct {
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		Location    struct {
			Name string `json:"name"`
		} `json:"location"`
		LocationType     string `json:"location_type"` // remote, hybrid, ...
		ContentPlain     string `json:"content_plain"`
		FirstPublishedAt string `json:"first_published_at"`
	}
	if err := getJSON(ctx, client, "https://api.gem.com/job_board/v0/"+url.PathEscape(company)+"/job_posts/", &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body))
	for _, j := range body {
		job := Job{Title: j.Title, Location: remote(j.LocationType == "remote", j.Location.Name), URL: j.AbsoluteURL, Posted: posted(j.FirstPublishedAt)}
		if text {
			job.text = j.ContentPlain
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
