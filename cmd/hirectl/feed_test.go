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
		{Feed{ID: "a", Title: "a", Boards: []string{"greenhouse:stripe"}}, true},
		{Feed{ID: "a", Title: "a", Boards: []string{"lever:netflix", "greenhouse: acme"}}, true},
		{Feed{ID: "a", Title: "", Boards: []string{"greenhouse:stripe"}}, false},
		{Feed{Title: "a"}, false},
		{Feed{ID: "a", Title: "a", Boards: []string{"stripe"}}, false},
		{Feed{ID: "a", Title: "a", Boards: []string{"greenhouse:"}}, false},
		{Feed{ID: "a", Title: "a", Boards: []string{"indeed:stripe"}}, false},
	} {
		if err := tc.feed.validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err = %v, want ok = %v", tc.feed, err, tc.ok)
		}
	}
}

func TestFeedMatches(t *testing.T) {
	f := Feed{Skills: []string{"Go", "C++"}, Location: "remote"}
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
