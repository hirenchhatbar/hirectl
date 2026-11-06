package main

import (
	"context"
	"net/http"
	"net/url"
)

func init() {
	integrations["ashby"] = &integration{fetch: fetchAshby, hosts: []string{"jobs.ashbyhq.com"}}
}

// fetchAshby reads a public Ashby job board: https://developers.ashbyhq.com/docs/public-job-posting-api
// The API always sends descriptions, so text only decides whether to keep them.
func fetchAshby(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body struct {
		Jobs []struct {
			Title            string `json:"title"`
			Location         string `json:"location"`
			JobURL           string `json:"jobUrl"`
			DescriptionPlain string `json:"descriptionPlain"`
			PublishedAt      string `json:"publishedAt"`
		} `json:"jobs"`
	}
	if err := getJSON(ctx, client, "https://api.ashbyhq.com/posting-api/job-board/"+url.PathEscape(company), &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		job := Job{Title: j.Title, Location: j.Location, URL: j.JobURL, Posted: posted(j.PublishedAt)}
		if text {
			job.text = j.DescriptionPlain
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
