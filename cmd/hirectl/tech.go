package main

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// techs lists the technologies discover tags jobs with: a name first, then other spellings found in
// job text. Matching is case-sensitive, as "Go" in prose ("go above and beyond") would mislead, and a
// word followed by "-" does not count, as in "Go-to-Market".
// shortcut: fixed list, add a name here when a feed skill is missing.
var techs = [][]string{
	{"Go", "Golang", "golang"},
	{"PHP"}, {"Laravel"}, {"Symfony"},
	{"Python"}, {"Django"}, {"Flask"}, {"FastAPI"},
	{"Java"}, {"Spring"}, {"Kotlin"}, {"Scala"},
	{"Ruby"}, {"Rails", "Ruby on Rails"},
	{"JavaScript", "Javascript"}, {"TypeScript", "Typescript"},
	{"Node.js", "NodeJS", "Node"}, {"React", "React.js"}, {"Vue", "Vue.js"}, {"Angular"},
	{"C#"}, {".NET", "dotnet"}, {"C++"}, {"Rust"}, {"Elixir"}, {"Swift"},
	{"AWS"}, {"GCP"}, {"Azure"}, {"Kubernetes", "k8s"}, {"Docker"}, {"Terraform"},
	{"PostgreSQL", "Postgres"}, {"MySQL"}, {"MongoDB"}, {"Redis"}, {"Kafka"},
}

// techIn returns the names of the techs whose spellings appear as words in text, in techs order.
func techIn(text string) []string {
	var found []string
	for _, t := range techs {
		if slices.ContainsFunc(t, func(s string) bool { return techRe(s).MatchString(text) }) {
			found = append(found, t[0])
		}
	}
	return found
}

// techRe matches spelling s as a word not followed by "-".
func techRe(s string) *regexp.Regexp { return wordRe("", s, `($|[^\pL\pN-])`) }

// techName returns the tech name for any of its spellings, case-insensitive, e.g. "golang" is "Go";
// a word not in techs is returned as is.
func techName(word string) string {
	for _, t := range techs {
		if slices.ContainsFunc(t, func(s string) bool { return strings.EqualFold(s, word) }) {
			return t[0]
		}
	}
	return word
}

// topTech returns up to n techs most common among jobs, ties by techs order.
func topTech(jobs []Job, n int) []string {
	count := map[string]int{}
	for _, j := range jobs {
		for _, t := range j.Tech {
			count[t]++
		}
	}
	order := func(name string) int { return slices.IndexFunc(techs, func(t []string) bool { return t[0] == name }) }
	top := slices.SortedFunc(maps.Keys(count), func(a, b string) int { return cmp.Or(count[b]-count[a], order(a)-order(b)) })
	return top[:min(n, len(top))]
}
