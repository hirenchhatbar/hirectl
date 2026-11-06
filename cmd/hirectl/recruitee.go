package main

import (
	"context"
	"net/http"
	"net/url"
)

func init() {
	integrations["recruitee"] = &integration{fetch: fetchRecruitee, hosts: []string{"*.recruitee.com"}}
}

// fetchRecruitee reads a company's public Recruitee offers: https://docs.recruitee.com/reference/offers
// The API always sends descriptions, so text only decides whether to keep them.
func fetchRecruitee(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var body struct {
		Offers []struct {
			Title        string `json:"title"`
			Location     string `json:"location"`
			Remote       bool   `json:"remote"`
			CareersURL   string `json:"careers_url"`
			Description  string `json:"description"`  // HTML
			Requirements string `json:"requirements"` // HTML
			PublishedAt  string `json:"published_at"` // e.g. 2026-10-30 16:32:58 UTC
		} `json:"offers"`
	}
	if err := getJSON(ctx, client, "https://api.recruitee.com/c/"+url.PathEscape(company)+"/careers/offers", &body); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(body.Offers))
	for _, j := range body.Offers {
		job := Job{Title: j.Title, Location: remote(j.Remote, j.Location), URL: j.CareersURL, Posted: posted(j.PublishedAt)}
		if text {
			job.text = j.Description + "\n" + j.Requirements
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
