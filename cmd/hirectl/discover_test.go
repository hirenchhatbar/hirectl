package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
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
		"com,recruitee,delta)/o/dev 1 {}\nio,greenhouse,boards)/gamma/jobs/1 1 {}\n")
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
	got, err := crawlBoards(context.Background(), client, []string{"lever", "recruitee"}, newProgress(io.Discard, io.Discard))
	if want := []string{"lever:acme", "lever:beta", "recruitee:delta"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
}

func TestOnlineBoardsListed(t *testing.T) {
	integrations["fa"] = &integration{boards: func(context.Context, *http.Client) ([]string, error) { return []string{"y", "x"}, nil }}
	defer delete(integrations, "fa")
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected GET %s: listed boards need no Common Crawl", r.URL)
		return nil, errors.New("no network")
	})}
	if got, err := onlineBoards(context.Background(), client, []string{"fa"}, newProgress(io.Discard, io.Discard)); err != nil || !slices.Equal(got, []string{"fa:x", "fa:y"}) {
		t.Fatalf("got %v, %v; want fa:x, fa:y", got, err)
	}
}

func TestCrawlCompany(t *testing.T) {
	for _, c := range []struct{ source, key, want string }{
		{"lever", "co,lever,jobs)/acme/1?x", "acme"},
		{"lever", "co,lever,jobs)/", ""},
		{"recruitee", "com,recruitee,acme)/o/dev", "acme"},
		{"recruitee", "com,recruitee,eu,acme)/", ""},
		{"personio", "com,personio,jobs,acme)/job/1", "acme"},
		{"workday", "com,myworkdayjobs,wd5,nvidia)/nvidiaexternalcareersite/job/x", "nvidia.wd5/nvidiaexternalcareersite"},
		{"workday", "com,myworkdayjobs,wd1,acme)/en-us/careers", "acme.wd1/careers"},
		{"workday", "com,myworkdayjobs,wd1,acme)/en-us", ""},
		{"workday", "com,myworkdayjobs,wd1,acme)/wday/cxs/acme/careers/jobs", ""},
		{"workday", "com,myworkdayjobs,www)/careers", ""},
	} {
		in := integrations[c.source]
		var got string
		for _, h := range in.hosts {
			if strings.HasPrefix(c.key, urlKey(h)) {
				got = crawlCompany(in, h, c.key)
			}
		}
		if got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.source, c.key, got, c.want)
		}
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
	got, failed := merge(stored, boards, res)
	want := []Job{
		{Board: "lever:flaky", Title: "Kept"},
		{Board: "lever:new", Title: "Go Dev", Location: "Remote"},
		{Board: "lever:other", Title: "Untouched"},
	}
	if failed != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %d failed; want %+v and 1 failed", got, failed, want)
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
	pickOf := func(picks []pick, s string) pick {
		t.Helper()
		i := slices.IndexFunc(picks, func(p pick) bool { return p.value == s })
		if i < 0 {
			t.Fatalf("no pick %s in %+v", s, picks)
		}
		return picks[i]
	}
	head, got := suggest(catalog, Feed{Skills: []string{"Golang"}, Locations: []string{"remote"}, Sources: []string{"greenhouse"}}, 10)
	if len(got) != len(integrations) || head != "Sources for Golang in remote: 3 boards match, run fetches the top 10 on the checked sources (none checked: all)" {
		t.Fatalf("got %q %+v, want every source; checked sources do not narrow the counts", head, got)
	}
	if p, want := pickOf(got, "lever"), (pick{"lever", "2 boards, 2 jobs", "", "Top boards: lever:a (1) · lever:e (1)"}); p != want {
		t.Fatalf("lever pick %+v, want %+v; Golang finds Go in titles and tags", p, want)
	}
	if p := pickOf(got, "greenhouse"); p.note != "1 boards, 2 jobs" {
		t.Fatalf("greenhouse pick %+v, want Golang to find Go in titles", p)
	}
	_, got = suggest(catalog, Feed{Skills: []string{"Go"}, Locations: []string{"remote"}}, 1)
	if p := pickOf(got, "greenhouse"); p.note != "1 boards, 2 jobs" || p.detail != "Top boards: greenhouse:b (2)" {
		t.Fatalf("greenhouse pick %+v, want greenhouse:b with 2 jobs", p)
	}
	if p := pickOf(got, "lever"); p.detail != "Top boards: lever:a (1)" {
		t.Fatalf("lever pick %+v, want only the top board, lever:a by name", p)
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
	if err := discoverCmd(context.Background(), t.TempDir(), []string{"--sources", "fa,fb", "--delay", "0s", "--boards", "fa:stripe, Acme Corp"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	slices.Sort(got) // printed as found, integrations in parallel
	if want := []string{"fa:acmecorp\t1 jobs\t", "fa:stripe\t1 jobs\t", "fb:acmecorp\t1 jobs\t"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	for _, b := range []string{"nope:stripe", "fa:"} {
		if err := discoverCmd(context.Background(), t.TempDir(), []string{"--boards", b}, io.Discard, io.Discard); err == nil {
			t.Errorf("--boards %q: want error", b)
		}
	}
}

func TestDiscoverCmdResume(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var probed []string
	integrations["fa"] = &integration{fetch: func(ctx context.Context, _ *http.Client, name string, _ bool) ([]Job, error) {
		if name == "b" {
			cancel() // interrupted on b
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		probed = append(probed, name)
		return []Job{{Title: "Dev"}}, nil
	}}
	defer delete(integrations, "fa")
	dir, args := t.TempDir(), []string{"--sources", "fa", "--delay", "0s", "--boards", "a,b,c"}
	if err := discoverCmd(ctx, dir, args, io.Discard, io.Discard); err == nil {
		t.Fatal("interrupted: want error")
	}
	other := discoverState{Sources: []string{"fb"}, Todo: []string{"fb:x"}} // another discover's, kept
	states, _ := loadJSON[[]discoverState](filepath.Join(dir, discoverFile))
	if len(states) != 1 || !slices.Equal(states[0].Todo, []string{"fa:b", "fa:c"}) {
		t.Fatalf("states = %v, want todo fa:b, fa:c", states)
	}
	if err := saveJSON(filepath.Join(dir, discoverFile), append(states, other)); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(dir, suggestionsFile), []Job{{Board: "fb:x", Title: "Ops"}, {Board: "fa:a", Title: "Dev"}}); err != nil {
		t.Fatal(err) // another discover saved fb:x meanwhile
	}
	if err := discoverCmd(context.Background(), dir, args, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(probed, []string{"a", "b", "c"}) {
		t.Errorf("probed %v, want a, then b and c on resume", probed)
	}
	if jobs, _ := loadJSON[[]Job](filepath.Join(dir, suggestionsFile)); len(jobs) != 4 {
		t.Errorf("stored %v, want fa:a, fa:b, fa:c and the other's fb:x", jobs)
	}
	if states, _ := loadJSON[[]discoverState](filepath.Join(dir, discoverFile)); !reflect.DeepEqual(states, []discoverState{other}) {
		t.Errorf("states = %v, want the other's only", states)
	}
}
