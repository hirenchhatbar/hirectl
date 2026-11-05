package main

import (
	"context"
	"net/http"
	"net/url"
)

func init() {
	integrations["lever"] = &integration{fetch: fetchLever, hosts: []string{"jobs.lever.co"}}
}

// fetchLever reads a public Lever postings list: https://github.com/lever/postings-api
// The API always sends descriptions, so text only decides whether to keep them.
func fetchLever(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body []struct {
		Text       string `json:"text"`
		HostedURL  string `json:"hostedUrl"`
		Categories struct {
			Location string `json:"location"`
		} `json:"categories"`
		DescriptionPlain string `json:"descriptionPlain"`
	}
	u := "https://api.lever.co/v0/postings/" + url.PathEscape(company) + "?mode=json"
	if err := getJSON(ctx, client, u, &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body))
	for _, j := range body {
		job := Job{Title: j.Text, Location: j.Categories.Location, URL: j.HostedURL}
		if text {
			job.text = j.DescriptionPlain
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
