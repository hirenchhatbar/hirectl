package main

import (
	"errors"
	"net/url"
	"strings"
)

// Source is a job website that feeds run against, e.g. https://www.indeed.com.
type Source struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

var sourceKind = kind[Source]{
	name:   "Source",
	file:   "sources.json",
	fields: []string{"Name", "URL"},
	hints:  []string{"Indeed", "https://www.indeed.com"},
	blank:  Source{Enabled: true},
	parse: func(v []string) (Source, error) {
		s := Source{Name: strings.TrimSpace(v[0]), URL: strings.TrimSpace(v[1]), Enabled: true}
		return s, s.validate()
	},
}

func (s Source) label() string    { return s.Name }
func (s Source) on() bool         { return s.Enabled }
func (s Source) values() []string { return []string{s.Name, s.URL} }

func (s Source) withOn(on bool) Source {
	s.Enabled = on
	return s
}

func (s Source) validate() error {
	if s.Name == "" {
		return errors.New("name is required")
	}
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("url must be http(s)://host, e.g. https://www.indeed.com")
	}
	return nil
}
