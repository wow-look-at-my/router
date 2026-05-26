package router

import (
	"net/url"
	"strings"
)

func splitPath(rawPath string) []string {
	if rawPath == "" || rawPath == "/" {
		return nil
	}
	s := rawPath
	if s[0] == '/' {
		s = s[1:]
	}
	if s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "/")
}

func tryMatch(p *pattern, pathSegs []string) (map[string]string, bool) {
	params := make(map[string]string)
	if matchSegments(p.segments, pathSegs, 0, 0, params, p.prefix) {
		return params, true
	}
	return nil, false
}

func matchSegments(pat []patternSeg, path []string, pi, si int, params map[string]string, prefix bool) bool {
	if pi >= len(pat) {
		if prefix {
			return true
		}
		return si >= len(path)
	}

	seg := pat[pi]

	switch seg.kind {
	case segLiteral:
		if si >= len(path) || path[si] != seg.value {
			return false
		}
		return matchSegments(pat, path, pi+1, si+1, params, prefix)

	case segParam:
		minNeeded := minRequired(pat, pi+1)
		maxSegs := len(path) - si - minNeeded
		if maxSegs < 1 {
			return false
		}
		for n := maxSegs; n >= 1; n-- {
			params[seg.value] = decodeSegments(path[si : si+n])
			if matchSegments(pat, path, pi+1, si+n, params, prefix) {
				return true
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
		return matchSegments(pat, path, pi+1, si+1, params, prefix)
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
