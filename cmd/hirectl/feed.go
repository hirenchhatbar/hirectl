package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// Feed is one saved job-post search and how often to run it.
type Feed struct {
	Title    string   `json:"title"`
	Skills   []string `json:"skills,omitempty"`
	Location string   `json:"location,omitempty"`
	Schedule string   `json:"schedule"` // cron expression or @daily, @weekly, ...
	Enabled  bool     `json:"enabled"`
}

var feedKind = kind[Feed]{
	name:   "Feed",
	file:   "feeds.json",
	fields: []string{"Title", "Skills", "Location", "Schedule"},
	hints:  []string{"", "Golang, PHP, Laravel", "Remote", "@daily, @weekly or 0 9 * * 1-5"},
	blank:  Feed{Schedule: "@daily", Enabled: true},
	parse: func(v []string) (Feed, error) {
		f := Feed{
			Title:    strings.TrimSpace(v[0]),
			Skills:   parseSkills(v[1]),
			Location: strings.TrimSpace(v[2]),
			Schedule: strings.TrimSpace(v[3]),
			Enabled:  true,
		}
		return f, f.validate()
	},
}

func (f Feed) label() string { return f.Title }
func (f Feed) on() bool      { return f.Enabled }

func (f Feed) withOn(on bool) Feed {
	f.Enabled = on
	return f
}

// values returns f's fields in feedKind.fields order, as the form edits them.
func (f Feed) values() []string {
	return []string{f.Title, strings.Join(f.Skills, ", "), f.Location, f.Schedule}
}

func (f Feed) validate() error {
	if f.Title == "" {
		return errors.New("title is required")
	}
	if _, err := cron.ParseStandard(f.Schedule); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	return nil
}

// parseSkills splits a comma-separated list, dropping blanks.
func parseSkills(s string) []string {
	var skills []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			skills = append(skills, p)
		}
	}
	return skills
}
