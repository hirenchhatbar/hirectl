package main

import "testing"

func TestLevel(t *testing.T) {
	for _, tc := range []struct {
		job  Job
		want string
	}{
		{Job{Title: "Engineer", text: "You have 5+ years of experience with Go."}, "5-7"},
		{Job{Title: "Senior Engineer", text: "3-5 years' professional experience"}, "3-4"},
		{Job{Title: "Engineer", text: "2 to 4 yrs experience"}, "0-2"},
		{Job{Title: "Engineer", text: "Founded 12 years ago. Experience in Go."}, ""},
		{Job{Title: "Mid-Senior Engineer"}, "5-7"},
		{Job{Title: "Staff Engineer", text: "10 years of hands-on experience"}, "8+"},
		{Job{Title: "Junior Developer"}, "0-2"},
		{Job{Title: "Engineer"}, ""},
	} {
		if got := level(tc.job); got != tc.want {
			t.Errorf("%+v: level = %q, want %q", tc.job, got, tc.want)
		}
	}
}
