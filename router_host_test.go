package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostMatchBindsDomainAndPath(t *testing.T) {
	r := New()
	var domain, path string
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		domain = req.PathValue("domain")
		path = req.PathValue("path")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dists/stable/InRelease", nil)
	req.Host = "apt.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "example.com", domain)
	require.Equal(t, "dists/stable/InRelease", path)
}

func TestHostMultiLabelDomain(t *testing.T) {
	r := New()
	var domain string
	r.HandleFunc("GET dl.{domain}/{project}", Allow, func(w http.ResponseWriter, req *http.Request) {
		domain = req.PathValue("domain")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/myapp", nil)
	req.Host = "dl.foo.pazer.build"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "foo.pazer.build", domain)
}

func TestHostPortStripped(t *testing.T) {
	r := New()
	var domain string
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		domain = req.PathValue("domain")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/x", nil)
	req.Host = "apt.localhost:8080"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "localhost", domain)
}

func TestHostAgnosticFallsThroughForUnknownSubdomain(t *testing.T) {
	r := New()
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201) // apt
	})
	r.HandleFunc("GET /healthz", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202) // bare-host healthz
	})

	// Unknown subdomain: host matches no host route -> host-agnostic eligible.
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "unknown.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 202, rec.Code)

	// Bare host: same, host-agnostic route serves.
	req = httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "example.com"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 202, rec.Code)
}

func TestHostClaimedShadowsHostAgnostic(t *testing.T) {
	r := New()
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201) // apt wildcard
	})
	r.HandleFunc("GET /healthz", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202) // bare-host healthz
	})

	// apt subdomain claims the host: even a path that a bare-host literal route
	// would match is served by apt's wildcard, never the host-agnostic route.
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "apt.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 201, rec.Code)
}

func TestHostClaimedNoPathMatchIs404(t *testing.T) {
	r := New()
	r.HandleFunc("GET sites.{domain}/{project}/branches", Allow, handler200)
	r.HandleFunc("GET /llms.txt", Allow, handler200)

	// sites claims the host but no sites route matches /llms.txt; it must 404,
	// not fall through to the host-agnostic /llms.txt.
	req := httptest.NewRequest("GET", "/llms.txt", nil)
	req.Host = "sites.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 404, rec.Code)

	// A different host is not claimed by sites -> /llms.txt serves.
	req = httptest.NewRequest("GET", "/llms.txt", nil)
	req.Host = "whatever.example.com"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
}

func TestHostLiteralOnly(t *testing.T) {
	r := New()
	r.HandleFunc("GET admin.example.com/panel", Allow, handler200)

	req := httptest.NewRequest("GET", "/panel", nil)
	req.Host = "admin.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)

	// A different domain does not match a fully-literal host.
	req = httptest.NewRequest("GET", "/panel", nil)
	req.Host = "admin.other.com"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 404, rec.Code)
}

func TestHostRoutesListing(t *testing.T) {
	r := New()
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, handler200)
	r.HandleFunc("GET /healthz", Allow, handler200)
	r.HandleFunc("HEAD oci.{domain}/v2/{$}", Allow, handler200)
	r.HandleFunc("GET oci.{domain}/v2/{$}", Allow, handler200)

	routes := r.Routes()
	require.Equal(t, 3, len(routes))
	// Sorted by full pattern: "/healthz" < "apt..." < "oci...".
	require.Equal(t, "/healthz", routes[0].Pattern)
	require.Equal(t, "apt.{domain}/{path...}", routes[1].Pattern)
	require.Equal(t, "apt.{domain}/{path...} {GET}", routes[1].String())
	require.Equal(t, "oci.{domain}/v2/{$}", routes[2].Pattern)
	require.Equal(t, "oci.{domain}/v2/{$} {GET,HEAD}", routes[2].String())
}

func TestHostLeadingParamBindsLabel(t *testing.T) {
	r := New()
	var project, path string
	r.HandleFunc("GET {project}.pazer.site/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		project = req.PathValue("project")
		path = req.PathValue("path")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/some/file.html", nil)
	req.Host = "tesla-wheel-data.pazer.site"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "tesla-wheel-data", project)
	require.Equal(t, "some/file.html", path)
}

func TestHostNonFinalParamExactlyOneLabel(t *testing.T) {
	r := New()
	r.HandleFunc("GET {project}.pazer.site/{path...}", Allow, handler200)

	// A non-final {project} matches exactly one label: a 4-label host does not
	// match the 3-label pattern, and neither does the bare 2-label domain.
	for _, host := range []string{"a.b.pazer.site", "pazer.site"} {
		req := httptest.NewRequest("GET", "/x", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, 404, rec.Code)
	}
}

func TestHostBareApexFallsThroughToHostAgnostic(t *testing.T) {
	r := New()
	r.HandleFunc("GET {project}.pazer.site/{path...}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201) // project site
	})
	r.HandleFunc("GET /__signin", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202) // apex route
	})

	// The bare 2-label apex has no label for {project} to bind, so it matches
	// no host-bearing route, the host stays unclaimed, and host-agnostic apex
	// routes serve it.
	req := httptest.NewRequest("GET", "/__signin", nil)
	req.Host = "pazer.site"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 202, rec.Code)
}

func TestHostNonFinalParamAccepted(t *testing.T) {
	// The pre-generalization parser panicked on a non-final host param (the old
	// TestHostWildcardNotLastPanics). It is now legal and binds exactly one label.
	r := New()
	var domain string
	r.HandleFunc("GET {domain}.apt/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		domain = req.PathValue("domain")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/x", nil)
	req.Host = "foo.apt"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "foo", domain)

	// Exactly one label: two labels before the literal do not match.
	req = httptest.NewRequest("GET", "/x", nil)
	req.Host = "foo.bar.apt"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 404, rec.Code)
}

func TestHostLiteralCountPrecedence(t *testing.T) {
	r := New()
	var which, project, domain string
	r.HandleFunc("GET dl.{domain}/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		which = "service"
		domain = req.PathValue("domain")
		w.WriteHeader(200)
	})
	r.HandleFunc("GET {project}.pazer.site/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		which = "project"
		project = req.PathValue("project")
		w.WriteHeader(200)
	})

	// dl.pazer.site matches both patterns; {project}.pazer.site has 2 literal
	// host labels vs dl.{domain}'s 1, so the project family wins wholesale.
	req := httptest.NewRequest("GET", "/anything", nil)
	req.Host = "dl.pazer.site"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "project", which)
	require.Equal(t, "dl", project)

	// On any other domain the service route is the only host match.
	which, domain = "", ""
	req = httptest.NewRequest("GET", "/anything", nil)
	req.Host = "dl.pazer.build"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "service", which)
	require.Equal(t, "pazer.build", domain)
}

func TestHostLiteralCountNeutralWithinFamily(t *testing.T) {
	r := New()
	var which string
	r.HandleFunc("GET {project}.pazer.site/dl/latest", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "literal"
		w.WriteHeader(200)
	})
	r.HandleFunc("GET {project}.pazer.site/dl/{version}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "version"
		w.WriteHeader(200)
	})

	// Identical host portions add the same host score: the existing path-based
	// best-match ordering (literal beats param) is unchanged within the family.
	req := httptest.NewRequest("GET", "/dl/latest", nil)
	req.Host = "myapp.pazer.site"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "literal", which)

	req = httptest.NewRequest("GET", "/dl/v1.0", nil)
	req.Host = "myapp.pazer.site"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "version", which)
}

func TestHostMultipleParams(t *testing.T) {
	r := New()
	var tenant, region string
	r.HandleFunc("GET {tenant}.{region}.example.com/x", Allow, func(w http.ResponseWriter, req *http.Request) {
		tenant = req.PathValue("tenant")
		region = req.PathValue("region")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/x", nil)
	req.Host = "acme.eu.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "acme", tenant)
	require.Equal(t, "eu", region)
}

func TestHostParamThenGreedyWildcard(t *testing.T) {
	r := New()
	var sub, domain string
	r.HandleFunc("GET {sub}.{domain}/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		sub = req.PathValue("sub")
		domain = req.PathValue("domain")
		w.WriteHeader(200)
	})

	// {sub} binds exactly one label; the final {domain} stays greedy.
	req := httptest.NewRequest("GET", "/x", nil)
	req.Host = "foo.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "foo", sub)
	require.Equal(t, "example.com", domain)

	sub, domain = "", ""
	req = httptest.NewRequest("GET", "/x", nil)
	req.Host = "a.b.example.com"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "a", sub)
	require.Equal(t, "b.example.com", domain)
}

func TestHostParamClaimsHost(t *testing.T) {
	r := New()
	r.HandleFunc("GET {project}.pazer.site/{path...}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201) // project site
	})
	r.HandleFunc("GET /healthz", Allow, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202) // bare-host healthz
	})

	// A host matched via a leading param claims the host like any other
	// host-bearing route: host-agnostic routes are ineligible.
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "myapp.pazer.site"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 201, rec.Code)

	// An unmatched host falls through to host-agnostic routes as usual.
	req = httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "myapp.other.site"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 202, rec.Code)
}

func TestHostParamOverridesSameNamePathParam(t *testing.T) {
	// A host param and a path param sharing a name keep the pre-existing
	// host-wildcard behavior: the host-bound value wins.
	r := New()
	var project string
	r.HandleFunc("GET {project}.example.com/dl/{project}", Allow, func(w http.ResponseWriter, req *http.Request) {
		project = req.PathValue("project")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dl/bar", nil)
	req.Host = "foo.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "foo", project)
}

func TestHostPartialLabelParamPanics(t *testing.T) {
	// Only whole-label host params are legal: partial-label and empty forms
	// are still rejected.
	for _, pat := range []string{
		"GET foo{x}bar.example.com/p",
		"GET {}.example.com/p",
		"GET {a{b}}.example.com/p",
	} {
		func() {
			defer func() { require.NotNil(t, recover()) }()
			r := New()
			r.HandleFunc(pat, Allow, handler200)
			t.Errorf("pattern %q did not panic", pat)
		}()
	}
}

func TestHostMethodNotAllowed(t *testing.T) {
	r := New()
	r.HandleFunc("GET apt.{domain}/{path...}", Allow, handler200)

	req := httptest.NewRequest("DELETE", "/x", nil)
	req.Host = "apt.example.com"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, 405, rec.Code)
	require.Equal(t, "GET, HEAD, OPTIONS", rec.Header().Get("Allow"))
}
