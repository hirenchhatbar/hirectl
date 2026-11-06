package main

import (
	"regexp"
	"slices"
	"strconv"
)

// levels group jobs by years of experience asked, fewest first: a name, its fewest years, and the title
// words that put a job in it when its description names no years.
var levels = []jobLevel{
	{"0-2", 0, []string{"Intern", "Internship", "Junior", "Jr", "Graduate", "Entry", "Trainee", "Apprentice"}},
	{"3-4", 3, []string{"Mid", "Medior", "Intermediate"}},
	{"5-7", 5, []string{"Senior", "Sr"}},
	{"8+", 8, []string{"Staff", "Principal", "Lead", "Architect", "Head", "Director", "Distinguished"}},
}

type jobLevel struct {
	name  string
	years int
	words []string
}

// yearsRe finds years of experience asked, as in "5+ years of experience", "3-5 years' professional
// experience" or "2 to 4 yrs experience"; the first number is the fewest years.
// shortcut: needs "experien" after the years in the same sentence, digits only ("five years" is missed),
// and "experience of 5 years" is missed; widen it when jobs show up without a level that state one.
var yearsRe = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])(\d{1,2})\s*\+?\s*(?:(?:-|–|to)\s*\d{1,2}\s*\+?\s*)?(?:years?|yrs?)\b[^.\n]{0,60}?experien`)

// level returns the name of j's level: from the first years of experience its description asks, else
// the highest level a title word names, else blank when unknown.
func level(j Job) string {
	m := yearsRe.FindStringSubmatch(j.text)
	for i := len(levels) - 1; i >= 0; i-- {
		l := levels[i]
		if m != nil {
			if y, _ := strconv.Atoi(m[1]); y >= l.years {
				return l.name
			}
		} else if slices.ContainsFunc(l.words, func(w string) bool { return wordRe("(?i)", w, techEnd).MatchString(j.Title) }) {
			return l.name
		}
	}
	return ""
}
