package router

import (
	"fmt"
	"strings"
)

type pattern struct {
	original         string
	method           string
	host             string // raw host portion, e.g. "apt.{domain}"; "" means host-agnostic
	hostSegs         []hostSeg
	hostParams       []string // names of host parameter labels, in label order
	hostLiteralCount int      // number of literal host labels
	path             string
	segments         []patternSeg
	queryParams      []queryParam
	prefix           bool
	exact            bool
	literalCount     int
}

// hostSeg is one dot-separated label of a pattern's host portion. A label is
// either a literal (wild == false) or a parameter (wild == true): a non-final
// parameter matches exactly one request-host label, while a final parameter is
// a wildcard capturing all remaining labels.
type hostSeg struct {
	wild  bool
	value string // literal label text, or parameter name
}

type segKind int

const (
	segLiteral segKind = iota
	segParam
	segWild
	segMixed
)

type patternSeg struct {
	kind  segKind
	value string    // literal text, param name, or wild name
	parts []mixPart // only for segMixed
}

type mixPart struct {
	literal string
	param   string
}

type queryParam struct {
	key   string
	param string
}

func parsePattern(s string) (*pattern, error) {
	p := &pattern{original: s}

	rest := s
	if idx := strings.IndexByte(s, ' '); idx >= 0 {
		p.method = s[:idx]
		rest = strings.TrimLeft(s[idx+1:], " ")
	}

	if idx := strings.IndexByte(rest, '?'); idx >= 0 {
		queryStr := rest[idx+1:]
		rest = rest[:idx]
		for _, part := range strings.Split(queryStr, "&") {
			if eq := strings.IndexByte(part, '='); eq >= 0 {
				key := part[:eq]
				val := part[eq+1:]
				if strings.HasPrefix(val, "{") && strings.HasSuffix(val, "}") {
					p.queryParams = append(p.queryParams, queryParam{
						key:   key,
						param: val[1 : len(val)-1],
					})
				}
			}
		}
	}

	// Optional host portion: a pattern that does not begin with "/" (after the
	// method) carries a host matcher before the path, e.g. "apt.{domain}/path".
	// The host portion is everything up to the first "/".
	if !strings.HasPrefix(rest, "/") {
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return nil, fmt.Errorf("pattern %q: host pattern must be followed by a path", s)
		}
		if err := p.parseHost(rest[:slash]); err != nil {
			return nil, fmt.Errorf("pattern %q: %w", s, err)
		}
		p.host = rest[:slash]
		rest = rest[slash:]
	}

	p.path = rest

	if len(rest) > 1 && rest[len(rest)-1] == '/' {
		p.prefix = true
	}

	raw := rest
	if len(raw) > 0 && raw[0] == '/' {
		raw = raw[1:]
	}
	if len(raw) > 0 && raw[len(raw)-1] == '/' {
		raw = raw[:len(raw)-1]
	}

	parts := strings.Split(raw, "/")
	if len(parts) == 1 && parts[0] == "" {
		parts = nil
	}

	if len(parts) > 0 && parts[len(parts)-1] == "{$}" {
		p.exact = true
		parts = parts[:len(parts)-1]
	}

	for _, part := range parts {
		seg, err := parseSegment(part)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", s, err)
		}
		p.segments = append(p.segments, seg)
		if seg.kind == segLiteral {
			p.literalCount++
		}
	}

	for i, seg := range p.segments {
		if seg.kind == segWild && i != len(p.segments)-1 {
			return nil, fmt.Errorf("pattern %q: wildcard must be last segment", s)
		}
	}

	return p, nil
}

// parseHost parses a pattern's host portion into dot-separated labels. Literal
// labels match the corresponding request-host label exactly. A whole-label
// "{name}" is a host parameter: in a non-final position it matches exactly one
// label; in the final position it is a wildcard capturing the remaining labels.
// Anything else containing "{" or "}" (e.g. a partial-label "foo{x}bar") is
// rejected.
func (p *pattern) parseHost(h string) error {
	if h == "" {
		return fmt.Errorf("empty host pattern")
	}
	labels := strings.Split(h, ".")
	for _, label := range labels {
		switch {
		case strings.HasPrefix(label, "{") && strings.HasSuffix(label, "}"):
			name := label[1 : len(label)-1]
			if name == "" {
				return fmt.Errorf("empty host parameter {} in %q", h)
			}
			if strings.ContainsAny(name, "{}") {
				return fmt.Errorf("invalid host parameter %q", label)
			}
			p.hostSegs = append(p.hostSegs, hostSeg{wild: true, value: name})
			p.hostParams = append(p.hostParams, name)
		case strings.ContainsAny(label, "{}"):
			return fmt.Errorf("invalid host label %q", label)
		default:
			p.hostSegs = append(p.hostSegs, hostSeg{value: label})
			p.hostLiteralCount++
		}
	}
	return nil
}

// full returns the pattern's host+path as registered. For host-agnostic
// patterns (no host portion) this is just the path.
func (p *pattern) full() string {
	return p.host + p.path
}

func parseSegment(s string) (patternSeg, error) {
	if !strings.Contains(s, "{") {
		return patternSeg{kind: segLiteral, value: s}, nil
	}

	if s[0] == '{' && s[len(s)-1] == '}' && strings.Count(s, "{") == 1 {
		name := s[1 : len(s)-1]
		if strings.HasSuffix(name, "...") {
			return patternSeg{kind: segWild, value: strings.TrimSuffix(name, "...")}, nil
		}
		return patternSeg{kind: segParam, value: name}, nil
	}

	var parts []mixPart
	remaining := s
	for len(remaining) > 0 {
		open := strings.IndexByte(remaining, '{')
		if open < 0 {
			parts = append(parts, mixPart{literal: remaining})
			break
		}
		if open > 0 {
			parts = append(parts, mixPart{literal: remaining[:open]})
		}
		close := strings.IndexByte(remaining[open:], '}')
		if close < 0 {
			return patternSeg{}, fmt.Errorf("unclosed { in segment %q", s)
		}
		close += open
		parts = append(parts, mixPart{param: remaining[open+1 : close]})
		remaining = remaining[close+1:]
	}

	return patternSeg{kind: segMixed, parts: parts}, nil
}

func (p *pattern) priority() int {
	// Literal host labels outrank every path term: of two host-bearing patterns
	// matching the same request, the one with more literal host labels wins
	// wholesale, so e.g. {project}.pazer.site (2 host literals) claims all of
	// *.pazer.site over dl.{domain} (1 host literal) regardless of path scores.
	// Patterns with identical host portions add the same constant, leaving the
	// existing path-based ordering untouched. The weight dominates any realistic
	// path score (path terms would need ~100 literal segments to reach it).
	score := p.hostLiteralCount * 1_000_000
	score += p.literalCount * 10000
	score += len(p.segments) * 100
	if p.exact {
		score += 50
	}
	if !p.prefix {
		score += 30
	}
	for _, seg := range p.segments {
		if seg.kind == segWild {
			score -= 1000
		}
		if seg.kind == segMixed {
			score += 50
		}
	}
	return score
}
