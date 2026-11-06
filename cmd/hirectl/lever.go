package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func init() {
	integrations["lever"] = &integration{fetch: fetchLever("https://api.lever.co"), hosts: []string{"jobs.lever.co"}}
	integrations["lever-eu"] = &integration{fetch: fetchLever("https://api.eu.lever.co"), hosts: []string{"jobs.eu.lever.co"}}
}

// fetchLever returns a fetch for a public Lever postings list on api, the global or EU instance:
// https://github.com/lever/postings-api
// The API always sends descriptions, so text only decides whether to keep them.
// It reads leverPage postings per request: a large board in one response (jobgether: 52 MB)
// outlasts the client timeout.
func fetchLever(api string) func(context.Context, *http.Client, string, bool) ([]Job, error) {
	return func(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
		var jobs []Job
		for {
			var body []struct {
				Text       string `json:"text"`
				HostedURL  string `json:"hostedUrl"`
				Categories struct {
					Location string `json:"location"`
				} `json:"categories"`
				DescriptionPlain string `json:"descriptionPlain"`
				CreatedAt        int64  `json:"createdAt"` // Unix ms
			}
			u := fmt.Sprintf("%s/v0/postings/%s?mode=json&limit=%d&skip=%d", api, url.PathEscape(company), leverPage, len(jobs))
			if err := getJSON(ctx, client, u, &body); err != nil {
				return nil, err
			}
			for _, j := range body {
				job := Job{Title: j.Text, Location: j.Categories.Location, URL: j.HostedURL}
				if j.CreatedAt > 0 {
					job.Posted = time.UnixMilli(j.CreatedAt).UTC()
				}
				if text {
					job.text = j.DescriptionPlain
				}
				jobs = append(jobs, job)
			}
			if len(body) < leverPage {
				return jobs, nil
			}
		}
	}
}

const leverPage = 100
