package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeClient answers every request with body, after checking its URL.
func fakeClient(t *testing.T, wantURL, body string) *http.Client {
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != wantURL {
			t.Errorf("GET %s, want %s", got, wantURL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestFetchGreenhouse(t *testing.T) {
	c := fakeClient(t, "https://boards-api.greenhouse.io/v1/boards/acme/jobs?content=true",
		`{"jobs":[{"title":"Go Dev","absolute_url":"https://x/1","location":{"name":"Remote"},"content":"&lt;p&gt;Go&lt;/p&gt;","first_published":"2026-09-03T13:30:34-04:00"}]}`)
	got, err := fetchGreenhouse(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1", Posted: time.Date(2026, 9, 3, 17, 30, 34, 0, time.UTC), text: "<p>Go</p>"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchGreenhouseNoText(t *testing.T) {
	c := fakeClient(t, "https://boards-api.greenhouse.io/v1/boards/acme/jobs", `{"jobs":[]}`)
	if _, err := fetchGreenhouse(context.Background(), c, "acme", false); err != nil {
		t.Fatal(err)
	}
}

func TestFetchLever(t *testing.T) {
	c := fakeClient(t, "https://api.lever.co/v0/postings/acme?mode=json&limit=100&skip=0",
		`[{"text":"Go Dev","hostedUrl":"https://x/1","categories":{"location":"Remote"},"descriptionPlain":"Go","createdAt":1763662158323}]`)
	got, err := fetchLever("https://api.lever.co")(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1", Posted: time.UnixMilli(1763662158323).UTC(), text: "Go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchLeverPages(t *testing.T) {
	full := "[" + strings.Repeat(`{"text":"Dev"},`, leverPage-1) + `{"text":"Dev"}]`
	pages := map[string]string{
		"https://api.lever.co/v0/postings/acme?mode=json&limit=100&skip=0":   full,
		"https://api.lever.co/v0/postings/acme?mode=json&limit=100&skip=100": `[{"text":"Last"}]`,
	}
	c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body, ok := pages[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected GET %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := fetchLever("https://api.lever.co")(context.Background(), c, "acme", false)
	if err != nil || len(got) != leverPage+1 || got[leverPage].Title != "Last" {
		t.Fatalf("got %d jobs, %v", len(got), err)
	}
}

func TestFetchAshby(t *testing.T) {
	c := fakeClient(t, "https://api.ashbyhq.com/posting-api/job-board/acme",
		`{"jobs":[{"title":"Go Dev","location":"Remote","jobUrl":"https://x/1","descriptionPlain":"Go","publishedAt":"2026-04-07T17:12:35.753+00:00"}]}`)
	got, err := fetchAshby(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1", Posted: time.Date(2026, 4, 7, 17, 12, 35, 753e6, time.UTC), text: "Go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchWorkable(t *testing.T) {
	c := fakeClient(t, "https://apply.workable.com/api/v1/widget/accounts/acme?details=true",
		`{"jobs":[{"title":"Go Dev","url":"https://x/1","city":"Athens","state":"","country":"Greece","telecommuting":true,"description":"<p>Go</p>","published_on":"2026-05-29"}]}`)
	got, err := fetchWorkable(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote, Athens, Greece", URL: "https://x/1", Posted: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), text: "<p>Go</p>"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchSmartRecruitersPages(t *testing.T) {
	pages := map[string]string{
		"https://api.smartrecruiters.com/v1/companies/acme/postings?limit=100&offset=0": `{"totalFound":2,"content":[{"id":"1","name":"Go Dev","location":{"fullLocation":"Berlin, Germany"},"releasedDate":"2026-11-06T07:58:45.807Z"}]}`,
		"https://api.smartrecruiters.com/v1/companies/acme/postings?limit=100&offset=1": `{"totalFound":2,"content":[{"id":"2","name":"Ops","location":{"fullLocation":"Remote"}}]}`,
	}
	c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body, ok := pages[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected GET %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := fetchSmartRecruiters(context.Background(), c, "acme", true)
	want := []Job{
		{Title: "Go Dev", Location: "Berlin, Germany", URL: "https://jobs.smartrecruiters.com/acme/1", Posted: time.Date(2026, 11, 6, 7, 58, 45, 807e6, time.UTC)},
		{Title: "Ops", Location: "Remote", URL: "https://jobs.smartrecruiters.com/acme/2"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchRippling(t *testing.T) {
	c := fakeClient(t, "https://api.rippling.com/platform/api/ats/v1/board/acme/jobs",
		`[{"name":" Go Dev ","url":"https://x/1","workLocation":{"label":"Berlin"}}]`)
	got, err := fetchRippling(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Berlin", URL: "https://x/1"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchRecruitee(t *testing.T) {
	c := fakeClient(t, "https://api.recruitee.com/c/acme/careers/offers",
		`{"offers":[{"title":"Go Dev","location":"Berlin, Germany","remote":true,"careers_url":"https://x/1","description":"<p>Go</p>","requirements":"<p>SQL</p>","published_at":"2026-10-30 16:32:58 UTC"}]}`)
	got, err := fetchRecruitee(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote, Berlin, Germany", URL: "https://x/1", Posted: time.Date(2026, 10, 30, 16, 32, 58, 0, time.UTC), text: "<p>Go</p>\n<p>SQL</p>"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchPersonio(t *testing.T) {
	c := fakeClient(t, "https://acme.jobs.personio.de/xml", `<?xml version="1.0" encoding="UTF-8"?><workzag-jobs><position>
<id> 7 </id><createdAt>2026-01-22T09:48:25+00:00</createdAt><office>Munich</office><additionalOffices><office>Berlin</office></additionalOffices><name>Go Dev</name>
<jobDescriptions><jobDescription><name>About</name><value><![CDATA[<p>Go</p>]]></value></jobDescription>
<jobDescription><name>You</name><value><![CDATA[<p>SQL</p>]]></value></jobDescription></jobDescriptions></position></workzag-jobs>`)
	got, err := fetchPersonio(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Munich; Berlin", URL: "https://acme.jobs.personio.de/job/7", Posted: time.Date(2026, 1, 22, 9, 48, 25, 0, time.UTC), text: "<p>Go</p>\n<p>SQL</p>"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchGem(t *testing.T) {
	c := fakeClient(t, "https://api.gem.com/job_board/v0/acme/job_posts/",
		`[{"title":"Go Dev","absolute_url":"https://x/1","location":{"name":"Amsterdam"},"location_type":"remote","content_plain":"Go","first_published_at":"2026-03-05T00:36:12.573Z"}]`)
	got, err := fetchGem(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote, Amsterdam", URL: "https://x/1", Posted: time.Date(2026, 3, 5, 0, 36, 12, 573e6, time.UTC), text: "Go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestFetchBreezy(t *testing.T) {
	c := fakeClient(t, "https://acme.breezy.hr/json",
		`[{"name":"Go Dev","url":"https://x/1","locations":[{"name":"Worldwide","is_remote":true},{"name":"Berlin"}],"published_date":"2024-02-15T14:37:22.684Z"}]`)
	got, err := fetchBreezy(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote, Worldwide; Berlin", URL: "https://x/1", Posted: time.Date(2024, 2, 15, 14, 37, 22, 684e6, time.UTC)}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestPosted(t *testing.T) {
	for in, want := range map[string]time.Time{
		"2026-09-03T13:30:34-04:00": time.Date(2026, 9, 3, 17, 30, 34, 0, time.UTC),
		"2026-10-30 16:32:58 UTC":   time.Date(2026, 10, 30, 16, 32, 58, 0, time.UTC),
		"2026-05-29":                time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC),
		"":                          {},
		"yesterday":                 {},
	} {
		if got := posted(in); !got.Equal(want) {
			t.Errorf("posted(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSiteURLNoLabel(t *testing.T) {
	for _, c := range []string{"nvidia.wd5/site", "a#b", ""} {
		if _, err := siteURL(c, "breezy.hr", "/json"); !errors.Is(err, errNotFound) {
			t.Errorf("siteURL(%q) = %v, want errNotFound", c, err)
		}
	}
}

func TestFetchWorkday(t *testing.T) {
	const base = "https://acme.wd5.myworkdayjobs.com/wday/cxs/acme/careers"
	full := `{"total":21,"jobPostings":[` + strings.Repeat(`{"externalPath":"/job/a"},`, workdayPage-1) + `{"externalPath":"/job/gone"}]}`
	answers := map[string]string{
		"POST " + base + "/jobs {\"limit\":20,\"offset\":0}":  full,
		"POST " + base + "/jobs {\"limit\":20,\"offset\":20}": `{"total":0,"jobPostings":[{"externalPath":"/job/b"}]}`,
		"GET " + base + "/job/a":                              `{"jobPostingInfo":{"title":"Go Dev","location":"Berlin","additionalLocations":["Remote"],"externalUrl":"https://x/a","jobDescription":"<p>Go</p>","startDate":"2026-11-06"}}`,
		"GET " + base + "/job/b":                              `{"jobPostingInfo":{"title":"Ops","location":"Paris","externalUrl":"https://x/b"}}`,
	}
	c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		req := r.Method + " " + r.URL.String()
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			req += strings.TrimRight(" "+string(b), " ")
		}
		if strings.HasSuffix(req, "/job/gone") {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
		}
		body, ok := answers[req]
		if !ok {
			t.Fatalf("unexpected %s", req)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := fetchWorkday(context.Background(), c, "acme.wd5/careers", true)
	if err != nil || len(got) != workdayPage {
		t.Fatalf("got %d jobs, %v; want %d", len(got), err, workdayPage)
	}
	if want := (Job{Title: "Go Dev", Location: "Berlin; Remote", URL: "https://x/a", Posted: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), text: "<p>Go</p>"}); !reflect.DeepEqual(got[0], want) {
		t.Errorf("first job %+v, want %+v", got[0], want)
	}
	if want := (Job{Title: "Ops", Location: "Paris", URL: "https://x/b"}); !reflect.DeepEqual(got[len(got)-1], want) {
		t.Errorf("last job %+v, want %+v", got[len(got)-1], want)
	}
}

func TestFetchWorkdayMissing(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusUnprocessableEntity} {
		c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: http.NoBody}, nil
		})}
		for _, board := range []string{"acme.wd5/gone", "acme"} {
			if _, err := fetchWorkday(context.Background(), c, board, true); !errors.Is(err, errNotFound) {
				t.Errorf("%d %s: got %v, want errNotFound", code, board, err)
			}
		}
	}
}

func TestFetchHirist(t *testing.T) {
	answers := map[string]string{
		"https://www.hirist.tech/c/backend-development-jobs":              `<script>{"slug":"backend-development-jobs","categoryId":"1"}</script>`,
		"https://gladiator.hirist.tech/job/category/?categoryId=1&page=0": `{"data":[{"id":1,"title":"Go Dev","createdTimeMs":1793690256693,"min":5,"locations":[{"name":"Remote"},{"name":"Bangalore"}],"tags":[{"name":"Golang"},{"name":"AWS"}]}],"hasMore":true}`,
		"https://gladiator.hirist.tech/job/category/?categoryId=1&page=1": `{"data":[{"id":2,"title":"Ops"}],"hasMore":false}`,
		"https://www.hirist.tech/c/stripe-jobs":                           `<script>{"slug":"stripe-jobs","categoryId":""}</script>`,
	}
	c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body, ok := answers[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected GET %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := fetchHirist(context.Background(), c, "backend-development", true)
	want := []Job{
		{Title: "Go Dev", Location: "Remote; Bangalore", URL: "https://www.hirist.tech/j/go-dev-1", Posted: time.UnixMilli(1793690256693).UTC(), text: "5 years of experience.\nGolang, AWS"},
		{Title: "Ops", URL: "https://www.hirist.tech/j/ops-2", text: "0 years of experience.\n"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
	if level(got[0]) != "5-7" || !reflect.DeepEqual(techIn(got[0].Title+"\n"+got[0].text), []string{"Go", "AWS"}) {
		t.Errorf("level %q, techs %v; want 5-7, [Go AWS]", level(got[0]), techIn(got[0].Title+"\n"+got[0].text))
	}
	for _, category := range []string{"stripe", "workday:a.wd5/b"} {
		if _, err := fetchHirist(context.Background(), c, category, true); !errors.Is(err, errNotFound) {
			t.Errorf("%s: got %v, want errNotFound", category, err)
		}
	}
}

func TestHiristBoards(t *testing.T) {
	c := fakeClient(t, "https://www.hirist.tech/new_sitemap-c-1.xml",
		`<urlset><url><loc>https://www.hirist.tech/c/backend-development-jobs</loc></url><url><loc>https://www.hirist.tech/c/ai-ml-jobs</loc></url></urlset>`)
	if got, err := hiristBoards(context.Background(), c); err != nil || !reflect.DeepEqual(got, []string{"backend-development", "ai-ml"}) {
		t.Fatalf("got %v, %v; want backend-development, ai-ml", got, err)
	}
}

func TestDescribeWorkday(t *testing.T) {
	c := fakeClient(t, "https://acme.wd5.myworkdayjobs.com/wday/cxs/acme/careers/job/Berlin/Go-Dev_R1", `{"jobPostingInfo":{"jobDescription":"<p>Go</p>"}}`)
	got, err := describeWorkday(context.Background(), c, "acme.wd5/careers", "https://acme.wd5.myworkdayjobs.com/en-US/careers/job/Berlin/Go-Dev_R1")
	if err != nil || got != "<p>Go</p>" {
		t.Fatalf("got %q, %v; want <p>Go</p>", got, err)
	}
}
