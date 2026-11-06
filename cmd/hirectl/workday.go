package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

func init() {
	integrations["workday"] = &integration{fetch: fetchWorkday, hosts: []string{"*.myworkdayjobs.com"}, company: workdayCompany}
}

// A Workday board is <tenant>.<wdN>/<site>, as in workday:nvidia.wd5/nvidiaexternalcareersite,
// from its page https://nvidia.wd5.myworkdayjobs.com/[<locale>/]NVIDIAExternalCareerSite/...
var (
	workdayHostRe   = regexp.MustCompile(`^([a-z0-9-]+)\.wd[0-9]+$`)
	workdayLocaleRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)
)

// workdayPage is the most postings Workday lists per request; more is a 400.
const workdayPage = 20

// workdayCompany returns the board of a crawled Workday page, or "" for none.
func workdayCompany(host, path string) string {
	host = strings.TrimSuffix(host, ".myworkdayjobs.com")
	site := firstSegment(path)
	if workdayLocaleRe.MatchString(site) {
		site = firstSegment(strings.TrimPrefix(path, "/"+site))
	}
	if !workdayHostRe.MatchString(host) || site == "" || site == "wday" { // /wday/ is the API
		return ""
	}
	return host + "/" + site
}

// fetchWorkday reads a public Workday career site through the JSON API its pages use: a POST per
// workdayPage postings, then a GET per posting, as only that holds its locations and description.
// So a board of n jobs takes n + n/20 requests, sent without pacing.
func fetchWorkday(ctx context.Context, client *http.Client, company string, text bool) ([]Job, error) {
	host, site, _ := strings.Cut(company, "/")
	m := workdayHostRe.FindStringSubmatch(host)
	if m == nil || site == "" { // as a bare --boards name
		return nil, fmt.Errorf("%q, want <tenant>.<wdN>/<site>: %w", company, errNotFound)
	}
	base := "https://" + host + ".myworkdayjobs.com/wday/cxs/" + m[1] + "/" + site
	var jobs []Job
	total := 0
	for offset := 0; ; offset += workdayPage {
		var page struct {
			Total       int `json:"total"` // set on the first page only
			JobPostings []struct {
				ExternalPath string `json:"externalPath"`
			} `json:"jobPostings"`
		}
		err := postJSON(ctx, client, base+"/jobs", map[string]int{"limit": workdayPage, "offset": offset}, &page)
		if se := (*statusError)(nil); errors.As(err, &se) && (se.code == http.StatusBadRequest || se.code == http.StatusUnprocessableEntity) {
			return nil, fmt.Errorf("%w: %w", errNotFound, err) // Workday answers a missing site 400, a missing tenant 422
		} else if err != nil {
			return nil, err
		}
		if offset == 0 {
			total = page.Total
		}
		for _, p := range page.JobPostings {
			var d struct {
				Info struct {
					Title               string   `json:"title"`
					Location            string   `json:"location"`
					AdditionalLocations []string `json:"additionalLocations"`
					ExternalURL         string   `json:"externalUrl"`
					JobDescription      string   `json:"jobDescription"` // HTML
					StartDate           string   `json:"startDate"`      // posting date, e.g. 2026-11-06
				} `json:"jobPostingInfo"`
			}
			err := getJSON(ctx, client, base+p.ExternalPath, &d)
			if errors.Is(err, errNotFound) { // closed since listed
				continue
			} else if err != nil {
				return nil, err
			}
			job := Job{Title: d.Info.Title, Location: locations(append([]string{d.Info.Location}, d.Info.AdditionalLocations...)), URL: d.Info.ExternalURL, Posted: posted(d.Info.StartDate)}
			if text {
				job.text = d.Info.JobDescription
			}
			jobs = append(jobs, job)
		}
		if len(page.JobPostings) < workdayPage || offset+workdayPage >= total {
			return jobs, nil
		}
	}
}
