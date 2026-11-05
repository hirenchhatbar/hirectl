package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestIndexBlocks(t *testing.T) {
	idx := strings.Join([]string{
		"co,acme)/a 1\tcdx-0.gz\t0\t10\t1",
		"co,ibm)/x 1\tcdx-0.gz\t10\t10\t2", // holds co,lever,jobs)/ keys up to the next line
		"co,microsoft)/x 1\tcdx-0.gz\t20\t10\t3",
		"io,greenhouse,boards)/a 1\tcdx-1.gz\t0\t5\t4", // the block before may hold earlier board keys
		"io,greenhouse,boards)/z 1\tcdx-1.gz\t5\t5\t5",
		"io,zz)/ 1\tcdx-1.gz\t10\t5\t6",
	}, "\n")
	got, err := indexBlocks(strings.NewReader(idx), []string{"co,lever,jobs)/", "io,greenhouse,boards)/"})
	want := []block{{"cdx-0.gz", 10, 10}, {"cdx-0.gz", 20, 10}, {"cdx-1.gz", 0, 5}, {"cdx-1.gz", 5, 5}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
	got, _ = indexBlocks(strings.NewReader(idx), []string{"org,zz)/"})
	if want := []block{{"cdx-1.gz", 10, 5}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("key after the last line: got %v, want %v", got, want)
	}
}

func gzipped(s string) []byte {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write([]byte(s))
	zw.Close()
	return b.Bytes()
}

func TestCrawlBoards(t *testing.T) {
	cdx := gzipped("co,lever,jobs)/acme/123 1 {}\nco,lever,jobs)/beta?x=1 1 {}\nco,lever,jobs)/acme 1 {}\nco,lever,jobs)/ 1 {}\n" +
		"io,greenhouse,boards)/gamma/jobs/1 1 {}\n")
	data := append([]byte("junk"), cdx...)
	base := crawlData + "cc-index/collections/CC-MAIN-1/indexes/"
	idx := fmt.Sprintf("co,lever,jobs)/acme 1\tcdx-0.gz\t4\t%d\t1\n", len(cdx))
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch u := r.URL.String(); u {
		case crawlList:
			body = []byte(`[{"id":"CC-MAIN-1"},{"id":"CC-MAIN-0"}]`)
		case base + "cluster.idx":
			body = []byte(idx)
		case base + "cdx-0.gz":
			var from, to int
			fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &from, &to)
			body = data[from : to+1]
		default:
			t.Errorf("unexpected GET %s", u)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	got, err := crawlBoards(context.Background(), client, []string{"lever"})
	if want := []string{"lever:acme", "lever:beta"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
}

func TestMerge(t *testing.T) {
	stored := []Job{
		{Board: "lever:gone", Title: "Old"},
		{Board: "lever:flaky", Title: "Kept"},
		{Board: "lever:other", Title: "Untouched"},
		{Board: "lever:new", Title: "Stale"},
	}
	boards := []string{"lever:flaky", "lever:gone", "lever:new"}
	res := []result{
		{err: fmt.Errorf("GET: 500")},
		{err: fmt.Errorf("GET: %w", errNotFound)},
		{jobs: []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1"}}},
	}
	got, errs := merge(stored, boards, res)
	want := []Job{
		{Board: "lever:flaky", Title: "Kept"},
		{Board: "lever:new", Title: "Go Dev", Location: "Remote"},
		{Board: "lever:other", Title: "Untouched"},
	}
	if len(errs) != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v and 1 error", got, errs, want)
	}
}

func TestSuggest(t *testing.T) {
	catalog := []Job{
		{Board: "lever:a", Title: "Go Dev", Location: "Remote"},
		{Board: "greenhouse:b", Title: "Go Dev", Location: "Remote"},
		{Board: "greenhouse:b", Title: "Senior Go Engineer", Location: "Remote EU"},
		{Board: "lever:c", Title: "PHP Dev", Location: "Remote"},
		{Board: "lever:d", Title: "Go Dev", Location: "Berlin"},
	}
	catalog[0].Tech = []string{"Go", "Kafka"}
	catalog = append(catalog, Job{Board: "lever:e", Title: "Backend Engineer", Location: "Remote", Tech: []string{"Go"}})
	head, got := suggest(catalog, Feed{Skills: []string{"Golang"}, Location: "remote"})
	want := []pick{
		{"lever:a", "1 of 1 jobs", "Go, Kafka", "Matching roles: Go Dev", true},
		{"lever:e", "1 of 1 jobs", "Go", "Matching roles: Backend Engineer", true},
		{"greenhouse:b", "0 of 2 jobs", "", "No matching roles", false},
		{"lever:c", "0 of 1 jobs", "", "No matching roles", false},
		{"lever:d", "0 of 1 jobs", "", "No matching roles", false},
	}
	if head != "Suggested boards for Golang in remote: 2 of 5 boards" || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q %+v, want %+v; Golang finds tagged Go jobs only", head, got, want)
	}
	_, got = suggest(catalog, Feed{Skills: []string{"Go"}, Location: "remote"})
	if got[0].value != "greenhouse:b" || got[0].note != "2 of 2 jobs" || got[0].detail != "Matching roles: Go Dev · Senior Go Engineer" {
		t.Fatalf("top pick %+v, want greenhouse:b with 2 of 2 jobs", got[0])
	}
}

func TestTechIn(t *testing.T) {
	got := techIn("Senior Engineer\n<p>Go-to-Market</p>")
	if got != nil {
		t.Fatalf("Go-to-Market tagged %v", got)
	}
	got = techIn("Senior Golang Engineer\n<p>We use PostgreSQL, Node.js and C++. Go above and beyond. Google. Go-to-Market.</p>")
	if want := []string{"Go", "Node.js", "C++", "PostgreSQL"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if techName("golang") != "Go" || techName("Laravel") != "Laravel" || techName("Cobol") != "Cobol" {
		t.Fatal("techName")
	}
	jobs := []Job{{Tech: []string{"Java", "Go"}}, {Tech: []string{"Go", "PHP"}}, {Tech: []string{"Rust"}}}
	if got := topTech(jobs, 3); !reflect.DeepEqual(got, []string{"Go", "PHP", "Java"}) {
		t.Fatalf("topTech = %v, want Go first, ties in techs order", got)
	}
}

func TestDiscoverCmdBoards(t *testing.T) {
	fake := &integration{fetch: func(context.Context, *http.Client, string, bool) ([]Job, error) { return []Job{{Title: "Dev"}}, nil }}
	integrations["fa"], integrations["fb"] = fake, fake
	defer delete(integrations, "fa")
	defer delete(integrations, "fb")
	var out strings.Builder
	if err := discoverCmd(context.Background(), t.TempDir(), []string{"--sources", "fa,fb", "--delay", "0s", "--boards", "fa:stripe, Acme Corp"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := "fa:acmecorp\t1 jobs\t\nfa:stripe\t1 jobs\t\nfb:acmecorp\t1 jobs\t\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
	for _, b := range []string{"nope:stripe", "fa:"} {
		if err := discoverCmd(context.Background(), t.TempDir(), []string{"--boards", b}, io.Discard); err == nil {
			t.Errorf("--boards %q: want error", b)
		}
	}
}
