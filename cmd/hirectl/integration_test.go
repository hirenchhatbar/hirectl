package main

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
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
		`{"jobs":[{"title":"Go Dev","absolute_url":"https://x/1","location":{"name":"Remote"},"content":"&lt;p&gt;Go&lt;/p&gt;"}]}`)
	got, err := fetchGreenhouse(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1", text: "<p>Go</p>"}}
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
	c := fakeClient(t, "https://api.lever.co/v0/postings/acme?mode=json",
		`[{"text":"Go Dev","hostedUrl":"https://x/1","categories":{"location":"Remote"},"descriptionPlain":"Go"}]`)
	got, err := fetchLever(context.Background(), c, "acme", true)
	want := []Job{{Title: "Go Dev", Location: "Remote", URL: "https://x/1", text: "Go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}
