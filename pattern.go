package router

import (
	"fmt"
	"strings"
)

type pattern struct {
	original     string
	method       string
	path         string
	segments     []patternSeg
	queryParams  []queryParam
	prefix       bool
	exact        bool
	literalCount int
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
	score := p.literalCount * 10000
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
