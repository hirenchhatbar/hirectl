package main

import (
	"reflect"
	"testing"
)

func TestFeedValidate(t *testing.T) {
	for _, tc := range []struct {
		feed Feed
		ok   bool
	}{
		{Feed{ID: "a", Title: "a"}, true},
		{Feed{ID: "a", Title: "a", Sources: []string{"lever", "greenhouse"}}, true},
		{Feed{ID: "a", Title: ""}, false},
		{Feed{Title: "a"}, false},
		{Feed{ID: "a", Title: "a", Sources: []string{"indeed"}}, false},
	} {
		if err := tc.feed.validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err = %v, want ok = %v", tc.feed, err, tc.ok)
		}
	}
}

func TestFeedMatches(t *testing.T) {
	f := Feed{Skills: []string{"Go", "C++"}, Locations: []string{"remote"}}
	for _, tc := range []struct {
		job Job
		ok  bool
	}{
		{Job{Title: "Senior Go Engineer", Location: "Remote - EU"}, true},
		{Job{Title: "Backend Engineer", Location: "Remote", text: "We use C++ daily."}, true},
		{Job{Title: "Go Engineer", Location: "Berlin"}, false},
		{Job{Title: "Google Ads Manager", Location: "Remote"}, false},
	} {
		if got := f.matches(tc.job); got != tc.ok {
			t.Errorf("%+v: matches = %v, want %v", tc.job, got, tc.ok)
		}
	}
	india := Feed{Locations: []string{"Remote Worldwide", "Remote India", "India"}}
	for loc, ok := range map[string]bool{
		"Remote, Worldwide": true, "Bangalore, India - Remote": true, "India (Remote)": true,
		"Bangalore, Karnataka, India": true, "Remote": false, "Remote US": false, "Indianapolis, IN": false,
	} {
		if got := india.matches(Job{Location: loc}); got != ok {
			t.Errorf("%q: matches = %v, want %v", loc, got, ok)
		}
	}
	if (Feed{Skills: []string{"Go"}}).matches(Job{Title: "Sales Manager", text: "Ready to go above and beyond?"}) {
		t.Error(`"go" in prose should not match skill Go`)
	}
	if !(Feed{}).matches(Job{Title: "anything"}) {
		t.Error("feed without skills or location should match every job")
	}
}

func TestParseList(t *testing.T) {
	got := parseList(" Golang, ,PHP ,")
	if want := []string{"Golang", "PHP"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := parseList("  "); got != nil {
		t.Fatalf("blank: got %q, want nil", got)
	}
}

func TestSlug(t *testing.T) {
	for title, want := range map[string]string{"Go remote (EU)": "go-remote-eu", "  C++ / PHP ": "c-php", "Ünïcode": "n-code", "": ""} {
		if got := slug(title); got != want {
			t.Errorf("slug(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestFeedScore(t *testing.T) {
	f := Feed{Skills: []string{"Go", "graphql"}}
	for _, tc := range []struct {
		job  Job
		want int
	}{
		{Job{Title: "Golang Lead"}, 10},
		{Job{Title: "Go-to-Market Lead", text: "go go Go"}, 1},
		{Job{Title: "Backend", text: "Go, Golang, Go, Go, Go, Go and GraphQL"}, 5 + 1},
		{Job{Title: "Backend", Tech: []string{"Go"}}, 1},
		{Job{Title: "Go Engineer", text: "We write Go."}, 11},
	} {
		if got := f.score(tc.job); got != tc.want {
			t.Errorf("%+v: score = %d, want %d", tc.job, got, tc.want)
		}
	}
}
