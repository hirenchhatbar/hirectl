package main

import "testing"

func TestSourceValidate(t *testing.T) {
	for _, tc := range []struct {
		src Source
		ok  bool
	}{
		{Source{Name: "Indeed", URL: "https://www.indeed.com"}, true},
		{Source{Name: "Naukri", URL: "http://naukri.com/jobs"}, true},
		{Source{Name: "", URL: "https://www.indeed.com"}, false},
		{Source{Name: "a", URL: ""}, false},
		{Source{Name: "a", URL: "indeed.com"}, false},
		{Source{Name: "a", URL: "ftp://indeed.com"}, false},
		{Source{Name: "a", URL: "https://"}, false},
	} {
		if err := tc.src.validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err = %v, want ok = %v", tc.src, err, tc.ok)
		}
	}
}
