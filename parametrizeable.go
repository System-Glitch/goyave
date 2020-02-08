package goyave

import (
	"fmt"
	"regexp"
	"strings"
)

// parametrizeable represents a route or router accepting
// parameters in its URI.
type parametrizeable struct {
	regex      *regexp.Regexp
	parameters []string
}

// compileParameters parse the route parameters and compiles their regexes if needed.
// If "ends" is set to true, the generated regex ends with "$", thus set "ends" to true
// if you're compiling route paramters, set to false if you're compiling router parameters.
func (p *parametrizeable) compileParameters(uri string, ends bool) {
	idxs, err := p.braceIndices(uri)
	if err != nil {
		panic(err)
	}

	var builder strings.Builder

	// Final regex will never be larger than src uri
	// Make initial alloc to avoid the need for realloc
	builder.Grow(len(uri))

	builder.WriteString("^")
	length := len(idxs)
	if length > 0 {
		end := 0
		for i := 0; i < length; i += 2 {
			raw := uri[end:idxs[i]]
			end = idxs[i+1]
			sub := uri[idxs[i]+1 : end]
			parts := strings.SplitN(sub, ":", 2)
			if parts[0] == "" {
				panic(fmt.Errorf("invalid route parameter, missing name in %q", sub))
			}
			pattern := "[^/]+" // default pattern
			if len(parts) == 2 {
				pattern = parts[1]
				if pattern == "" {
					panic(fmt.Errorf("invalid route parameter, missing pattern in %q", sub))
				}
			}

			builder.WriteString(raw)
			builder.WriteString("(")
			builder.WriteString(pattern)
			builder.WriteString(")")
			end++ // Skip closing braces
			p.parameters = append(p.parameters, parts[0])
		}
		builder.WriteString(uri[end:])
	} else {
		builder.WriteString(uri)
	}

	if ends {
		builder.WriteString("$")
	}

	p.regex = regexp.MustCompile(builder.String()) // TODO cache recurrent regexes

	if p.regex.NumSubexp() != length/2 {
		panic(fmt.Sprintf("route %s contains capture groups in its regexp. ", uri) +
			"Only non-capturing groups are accepted: e.g. (?:pattern) instead of (pattern)")
	}
}

// braceIndices returns the first level curly brace indices from a string.
// Returns an error in case of unbalanced braces.
func (p *parametrizeable) braceIndices(s string) ([]int, error) {
	var level, idx int
	indices := make([]int, 0, 2)
	length := len(s)
	for i := 0; i < length; i++ {
		if s[i] == '{' {
			level++
			if level == 1 {
				idx = i
			}
		} else if s[i] == '}' {
			level--
			if level == 0 {
				if i == idx+1 {
					return nil, fmt.Errorf("empty route parameter in %q", s)
				}
				indices = append(indices, idx, i)
			} else if level < 0 {
				return nil, fmt.Errorf("unbalanced braces in %q", s)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("unbalanced braces in %q", s)
	}
	return indices, nil
}

// makeParameters from a regex match and the given parameter names.
// The match parameter is expected to contain only the capturing groups.
// The full match should be excluded. The two given slices are expected to
// have the same size.
//
//  p.makeParameters(matches[1:])
//
// Given ["33", "param"] ["id", "name"]
// The returned map will be ["id": "33", "name": "param"]
func (p *parametrizeable) makeParameters(match []string, names []string) map[string]string {
	params := make(map[string]string, len(match))
	for i, v := range match {
		params[names[i]] = v
	}
	return params
}