package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func init() {
	integrations["hirist"] = &integration{fetch: fetchHirist, boards: hiristBoards}
}

// hirist.tech is a job site, not a company's board: a hirist board is one of its job categories, named by
// the slug of its page https://www.hirist.tech/c/<slug>-jobs, as in hirist:backend-development.
const (
	hiristSite = "https://www.hirist.tech"
	hiristAPI  = "https://gladiator.hirist.tech" // the undocumented JSON API the site's pages use
)

var (
	hiristCategoryRe = regexp.MustCompile(`https://www\.hirist\.tech/c/([a-z0-9-]+)-jobs<`) // a category page in the sitemap
	hiristIDRe       = regexp.MustCompile(`"categoryId":"(\d+)"`)                           // a category page's ID, in its page data
)

// hiristBoards lists the categories in the site's category sitemap.
func hiristBoards(ctx context.Context, client *http.Client) ([]string, error) {
	page, err := getText(ctx, client, hiristSite+"/new_sitemap-c-1.xml")
	if err != nil {
		return nil, err
	}
	var boards []string
	for _, m := range hiristCategoryRe.FindAllStringSubmatch(page, -1) {
		boards = append(boards, m[1])
	}
	return boards, nil
}

// fetchHirist reads a category's jobs: its page for its ID, then the API, 50 jobs per page.
// So a category of n jobs takes 1 + n/50 requests, sent without pacing.
// The list has no description: when text is true, a job's text is its years of experience asked and its
// skill tags, which is all run and discover read a description for (level, score, techs).
func fetchHirist(ctx context.Context, client *http.Client, category string, text bool) ([]Job, error) {
	if !labelRe.MatchString(category) {
		return nil, fmt.Errorf("%q: %w", category, errNotFound)
	}
	page, err := getText(ctx, client, hiristSite+"/c/"+category+"-jobs")
	if err != nil {
		return nil, err
	}
	m := hiristIDRe.FindStringSubmatch(page)
	if m == nil { // the site answers an unknown category with an empty page
		return nil, fmt.Errorf("category %q: %w", category, errNotFound)
	}
	var jobs []Job
	for n := 0; ; n++ {
		var body struct {
			Data []struct {
				ID        int    `json:"id"`
				Title     string `json:"title"`
				Created   int64  `json:"createdTimeMs"`
				Min       int    `json:"min"` // fewest years of experience asked
				Locations []struct {
					Name string `json:"name"`
				} `json:"locations"`
				Tags []struct {
					Name string `json:"name"`
				} `json:"tags"`
			} `json:"data"`
			HasMore bool `json:"hasMore"`
		}
		if err := getJSON(ctx, client, fmt.Sprintf("%s/job/category/?categoryId=%s&page=%d", hiristAPI, m[1], n), &body); err != nil {
			return nil, err
		}
		for _, j := range body.Data {
			var locs, tags []string
			for _, l := range j.Locations {
				locs = append(locs, l.Name)
			}
			for _, t := range j.Tags {
				tags = append(tags, t.Name)
			}
			// the list has no URL; a job page reads the ID ending its path, the rest is for show
			job := Job{Title: j.Title, Location: locations(locs), URL: fmt.Sprintf("%s/j/%s-%d", hiristSite, slug(j.Title), j.ID)}
			if j.Created > 0 {
				job.Posted = time.UnixMilli(j.Created).UTC()
			}
			if text {
				job.text = fmt.Sprintf("%d years of experience.\n%s", j.Min, strings.Join(tags, ", "))
			}
			jobs = append(jobs, job)
		}
		if !body.HasMore || len(body.Data) == 0 {
			return jobs, nil
		}
	}
}

// getText GETs url and returns its body.
func getText(ctx context.Context, client *http.Client, url string) (string, error) {
	body, err := get(ctx, client, url, "")
	if err != nil {
		return "", err
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	return string(b), err
}
