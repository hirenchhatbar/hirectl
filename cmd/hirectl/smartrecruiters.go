package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func init() {
	integrations["smartrecruiters"] = &integration{fetch: fetchSmartRecruiters, hosts: []string{"jobs.smartrecruiters.com"}}
}

// fetchSmartRecruiters reads a company's public SmartRecruiters postings, 100 per page:
// https://developers.smartrecruiters.com/docs/posting-api
// shortcut: no descriptions (text is ignored), as the list has none and each needs its own request;
// discover tags techs from titles only. Fetch /postings/<id> per job if title tags prove too thin.
func fetchSmartRecruiters(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	var jobs []Job
	for {
		var body struct {
			TotalFound int `json:"totalFound"`
			Content    []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Location struct {
					FullLocation string `json:"fullLocation"`
				} `json:"location"`
				ReleasedDate string `json:"releasedDate"`
			} `json:"content"`
		}
		u := fmt.Sprintf("https://api.smartrecruiters.com/v1/companies/%s/postings?limit=100&offset=%d", url.PathEscape(company), len(jobs))
		if err := getJSON(ctx, client, u, &body); err != nil {
			return nil, err
		}
		for _, j := range body.Content {
			jobs = append(jobs, Job{Title: j.Name, Location: j.Location.FullLocation,
				URL: "https://jobs.smartrecruiters.com/" + url.PathEscape(company) + "/" + url.PathEscape(j.ID), Posted: posted(j.ReleasedDate)})
		}
		if len(body.Content) == 0 || len(jobs) >= body.TotalFound {
			return jobs, nil
		}
	}
}
