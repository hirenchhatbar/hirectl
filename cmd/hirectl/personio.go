package main

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
)

// Personio serves each company's board on both domains; jobs are read from the first.
func init() {
	integrations["personio"] = &integration{fetch: fetchPersonio, hosts: []string{"*.jobs.personio.de", "*.jobs.personio.com"}}
}

// fetchPersonio reads a company's public Personio XML feed: https://developer.personio.de/docs/retrieving-open-job-positions
// The feed always holds descriptions, so text only decides whether to keep them.
func fetchPersonio(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	base, err := siteURL(company, "jobs.personio.de", "")
	if err != nil {
		return nil, err
	}
	body, err := get(ctx, client, base+"/xml", "")
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var feed struct {
		Positions []struct {
			ID           string   `xml:"id"`
			Name         string   `xml:"name"`
			Office       string   `xml:"office"`
			Offices      []string `xml:"additionalOffices>office"`
			Descriptions []string `xml:"jobDescriptions>jobDescription>value"` // HTML
			CreatedAt    string   `xml:"createdAt"`
		} `xml:"position"`
	}
	if err := xml.NewDecoder(body).Decode(&feed); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(feed.Positions))
	for _, j := range feed.Positions {
		job := Job{Title: strings.TrimSpace(j.Name), Location: locations(append([]string{j.Office}, j.Offices...)),
			URL: base + "/job/" + url.PathEscape(strings.TrimSpace(j.ID)), Posted: posted(strings.TrimSpace(j.CreatedAt))}
		if text {
			job.text = strings.Join(j.Descriptions, "\n")
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
