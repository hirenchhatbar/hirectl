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
		{Feed{Title: "a", Schedule: "@daily"}, true},
		{Feed{Title: "a", Schedule: "@weekly"}, true},
		{Feed{Title: "a", Schedule: "0 9 * * 1-5"}, true},
		{Feed{Title: "", Schedule: "@daily"}, false},
		{Feed{Title: "a", Schedule: ""}, false},
		{Feed{Title: "a", Schedule: "daily"}, false},
		{Feed{Title: "a", Schedule: "61 * * * *"}, false},
	} {
		if err := tc.feed.validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err = %v, want ok = %v", tc.feed, err, tc.ok)
		}
	}
}

func TestParseSkills(t *testing.T) {
	got := parseSkills(" Golang, ,PHP ,")
	if want := []string{"Golang", "PHP"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := parseSkills("  "); got != nil {
		t.Fatalf("blank: got %q, want nil", got)
	}
}
