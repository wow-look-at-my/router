package router

import (
	"net/url"
	"strings"
)

// splitHost returns the dot-separated labels of a request host, with any port
// and surrounding dots stripped. "apt.example.com:8080" -> ["apt","example","com"].
func splitHost(host string) []string {
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.Trim(host, ".")
	if host == "" {
		return nil
	}
	return strings.Split(host, ".")
}

// matchHostSegs reports whether a pattern's host labels match the request host
// labels. On success, values holds the text captured by each parameter label in
// label order: a non-final parameter captures exactly the one label at its
// position, and a final parameter captures all remaining labels (joined by ".").
func matchHostSegs(segs []hostSeg, hostLabels []string) (values []string, ok bool) {
	n := len(segs)
	if n == 0 {
		return nil, true
	}
	greedy := segs[n-1].wild
	fixed := n // labels matched one-to-one (literals and non-final parameters)
	if greedy {
		fixed = n - 1
		if len(hostLabels) <= fixed { // the trailing wildcard needs at least one label
			return nil, false
		}
	} else if len(hostLabels) != fixed {
		return nil, false
	}
	for i := 0; i < fixed; i++ {
		if !segs[i].wild && segs[i].value != hostLabels[i] {
			return nil, false
		}
	}
	for i := 0; i < fixed; i++ {
		if segs[i].wild {
			values = append(values, hostLabels[i])
		}
	}
	if greedy {
		values = append(values, strings.Join(hostLabels[fixed:], "."))
	}
	return values, true
}

func splitPath(rawPath string) []string {
	if rawPath == "" || rawPath == "/" {
		return nil
	}
	s := rawPath
	if s[0] == '/' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "/")
}

func hasTrailingSlash(rawPath string) bool {
	return len(rawPath) > 0 && rawPath[len(rawPath)-1] == '/'
}

func tryMatch(p *pattern, pathSegs []string, trailing bool) (map[string]string, bool) {
	params := make(map[string]string)
	if matchSegments(p.segments, pathSegs, 0, 0, params, p.prefix, p.exact, trailing) {
		return params, true
	}
	return nil, false
}

func matchSegments(pat []patternSeg, path []string, pi, si int, params map[string]string, prefix, exact, trailing bool) bool {
	if pi >= len(pat) {
		if si < len(path) {
			return prefix
		}
		if prefix || exact {
			return trailing
		}
		return true
	}

	seg := pat[pi]

	switch seg.kind {
	case segLiteral:
		if si >= len(path) || path[si] != seg.value {
			return false
		}
		return matchSegments(pat, path, pi+1, si+1, params, prefix, exact, trailing)

	case segParam:
		minNeeded := minRequired(pat, pi+1)
		maxSegs := len(path) - si - minNeeded
		if maxSegs < 1 {
			return false
		}
		if hasWildAfter(pat, pi+1) {
			for n := 1; n <= maxSegs; n++ {
				params[seg.value] = decodeSegments(path[si : si+n])
				if matchSegments(pat, path, pi+1, si+n, params, prefix, exact, trailing) {
					return true
				}
			}
		} else {
			for n := maxSegs; n >= 1; n-- {
				params[seg.value] = decodeSegments(path[si : si+n])
				if matchSegments(pat, path, pi+1, si+n, params, prefix, exact, trailing) {
					return true
				}
			}
		}
		delete(params, seg.value)
		return false

	case segWild:
		params[seg.value] = decodeSegments(path[si:])
		return true

	case segMixed:
		if si >= len(path) {
			return false
		}
		decoded, err := url.PathUnescape(path[si])
		if err != nil {
			decoded = path[si]
		}
		if !matchMixed(seg.parts, decoded, params) {
			return false
		}
		return matchSegments(pat, path, pi+1, si+1, params, prefix, exact, trailing)
	}

	return false
}

func minRequired(pat []patternSeg, from int) int {
	n := 0
	for i := from; i < len(pat); i++ {
		if pat[i].kind != segWild {
			n++
		}
	}
	return n
}

func hasWildAfter(pat []patternSeg, from int) bool {
	for i := from; i < len(pat); i++ {
		if pat[i].kind == segWild {
			return true
		}
	}
	return false
}

func decodeSegments(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	parts := make([]string, len(segs))
	for i, s := range segs {
		d, err := url.PathUnescape(s)
		if err != nil {
			d = s
		}
		parts[i] = d
	}
	return strings.Join(parts, "/")
}

func matchMixed(parts []mixPart, text string, params map[string]string) bool {
	return matchMixedAt(parts, text, 0, params)
}

func matchMixedAt(parts []mixPart, text string, pos int, params map[string]string) bool {
	if len(parts) == 0 {
		return pos == len(text)
	}

	p := parts[0]
	rest := parts[1:]

	if p.literal != "" {
		if !strings.HasPrefix(text[pos:], p.literal) {
			return false
		}
		return matchMixedAt(rest, text, pos+len(p.literal), params)
	}

	if len(rest) == 0 {
		if pos >= len(text) {
			return false
		}
		params[p.param] = text[pos:]
		return true
	}

	for end := len(text); end > pos; end-- {
		params[p.param] = text[pos:end]
		if matchMixedAt(rest, text, end, params) {
			return true
		}
	}
	delete(params, p.param)
	return false
}
