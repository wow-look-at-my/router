package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func handler200(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestBasicRouting(t *testing.T) {
	r := New()
	r.HandleFunc("GET /hello", Allow, handler200)

	req := httptest.NewRequest("GET", "/hello", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMultiSegmentParam(t *testing.T) {
	r := New()
	var got string
	r.HandleFunc("GET /dl/{project}/latest/{os}/{arch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		got = req.PathValue("project")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dl/my-org/my-project/latest/linux/amd64", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "my-org/my-project" {
		t.Fatalf("expected project=%q, got %q", "my-org/my-project", got)
	}
}

func TestPercentEncodedSlash(t *testing.T) {
	r := New()
	var got string
	r.HandleFunc("GET /dl/{project}/latest/{os}/{arch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		got = req.PathValue("project")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dl/my-org%2Fmy-project/latest/linux/amd64", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "my-org/my-project" {
		t.Fatalf("expected project=%q, got %q", "my-org/my-project", got)
	}
}

func TestAmbiguousBacktrack(t *testing.T) {
	r := New()
	var gotProject, gotOS, gotArch string
	r.HandleFunc("GET /dl/{project}/latest/{os}/{arch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		gotProject = req.PathValue("project")
		gotOS = req.PathValue("os")
		gotArch = req.PathValue("arch")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dl/my-org/latest/latest/linux/amd64", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotProject != "my-org/latest" {
		t.Fatalf("expected project=%q, got %q", "my-org/latest", gotProject)
	}
	if gotOS != "linux" {
		t.Fatalf("expected os=%q, got %q", "linux", gotOS)
	}
	if gotArch != "amd64" {
		t.Fatalf("expected arch=%q, got %q", "amd64", gotArch)
	}
}

func TestOCIActionKeywordInName(t *testing.T) {
	r := New()
	var gotProject, gotRef string
	r.HandleFunc("GET /v2/{project}/manifests/{reference}", Allow, func(w http.ResponseWriter, req *http.Request) {
		gotProject = req.PathValue("project")
		gotRef = req.PathValue("reference")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/v2/foo/manifests/manifests/sha256:abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotProject != "foo/manifests" {
		t.Fatalf("expected project=%q, got %q", "foo/manifests", gotProject)
	}
	if gotRef != "sha256:abc" {
		t.Fatalf("expected reference=%q, got %q", "sha256:abc", gotRef)
	}
}

func TestConsecutiveParams(t *testing.T) {
	r := New()
	var a, b, c, d string
	r.HandleFunc("GET /{a}/{b}/{c}/{d}", Allow, func(w http.ResponseWriter, req *http.Request) {
		a = req.PathValue("a")
		b = req.PathValue("b")
		c = req.PathValue("c")
		d = req.PathValue("d")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/a/b/c/d/e/f", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if a != "a/b/c" || b != "d" || c != "e" || d != "f" {
		t.Fatalf("expected a=a/b/c b=d c=e d=f, got a=%q b=%q c=%q d=%q", a, b, c, d)
	}
}

func TestBestMatch(t *testing.T) {
	r := New()
	var which string
	r.HandleFunc("GET /dl/{project}/latest/{os}/{arch}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "latest"
		w.WriteHeader(200)
	})
	r.HandleFunc("GET /dl/{project}/{version}/{os}/{arch}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "version"
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/dl/myapp/latest/linux/amd64", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if which != "latest" {
		t.Fatalf("expected latest route, got %q", which)
	}

	which = ""
	req = httptest.NewRequest("GET", "/dl/myapp/v1.0/linux/amd64", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if which != "version" {
		t.Fatalf("expected version route, got %q", which)
	}
}

func TestIntraSegmentSuffix(t *testing.T) {
	r := New()
	var got string
	r.HandleFunc("GET /brew/{project}.rb", Allow, func(w http.ResponseWriter, req *http.Request) {
		got = req.PathValue("project")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/brew/myapp.rb", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "myapp" {
		t.Fatalf("expected project=%q, got %q", "myapp", got)
	}
}

func TestIntraSegmentMultiParam(t *testing.T) {
	r := New()
	var project, os, arch string
	r.HandleFunc("GET /npm/@buildhost/{project}-{os}-{arch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		project = req.PathValue("project")
		os = req.PathValue("os")
		arch = req.PathValue("arch")
		w.WriteHeader(200)
	})

	tests := []struct {
		path                        string
		wantProject, wantOS, wantArch string
	}{
		{"/npm/@buildhost/myapp-linux-x64", "myapp", "linux", "x64"},
		{"/npm/@buildhost/my-cool-app-linux-x64", "my-cool-app", "linux", "x64"},
	}

	for _, tt := range tests {
		project, os, arch = "", "", ""
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Fatalf("%s: expected 200, got %d", tt.path, rec.Code)
		}
		if project != tt.wantProject || os != tt.wantOS || arch != tt.wantArch {
			t.Fatalf("%s: expected project=%q os=%q arch=%q, got project=%q os=%q arch=%q",
				tt.path, tt.wantProject, tt.wantOS, tt.wantArch, project, os, arch)
		}
	}
}

func TestRoutesConsolidation(t *testing.T) {
	r := New()
	r.HandleFunc("GET /foo", Allow, handler200)
	r.HandleFunc("PUT /foo", Allow, handler200)
	r.HandleFunc("DELETE /foo", Allow, handler200)
	r.HandleFunc("/bar/", Allow, handler200)

	routes := r.Routes()

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d: %v", len(routes), routes)
	}

	if routes[0].Pattern != "/bar/" {
		t.Fatalf("expected first route /bar/, got %q", routes[0].Pattern)
	}
	if routes[0].Methods != nil {
		t.Fatalf("expected nil methods for /bar/, got %v", routes[0].Methods)
	}
	if routes[0].String() != "/bar/ {*}" {
		t.Fatalf("expected '/bar/ {*}', got %q", routes[0].String())
	}

	if routes[1].Pattern != "/foo" {
		t.Fatalf("expected second route /foo, got %q", routes[1].Pattern)
	}
	if len(routes[1].Methods) != 3 || routes[1].Methods[0] != "DELETE" || routes[1].Methods[1] != "GET" || routes[1].Methods[2] != "PUT" {
		t.Fatalf("expected methods [DELETE,GET,PUT], got %v", routes[1].Methods)
	}
	if routes[1].String() != "/foo {DELETE,GET,PUT}" {
		t.Fatalf("expected '/foo {DELETE,GET,PUT}', got %q", routes[1].String())
	}
}

func Test405WithAllow(t *testing.T) {
	r := New()
	r.HandleFunc("GET /resource", Allow, handler200)
	r.HandleFunc("PUT /resource", Allow, handler200)

	req := httptest.NewRequest("DELETE", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 405 {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	if allow != "GET, PUT" {
		t.Fatalf("expected Allow: GET, PUT, got %q", allow)
	}
}

func Test404(t *testing.T) {
	r := New()
	r.HandleFunc("GET /exists", Allow, handler200)

	req := httptest.NewRequest("GET", "/nope", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPrefixMatch(t *testing.T) {
	r := New()
	r.HandleFunc("/v2/", Allow, handler200)

	tests := []struct {
		path string
		want int
	}{
		{"/v2/", 200},
		{"/v2/foo/bar", 200},
		{"/v2", 404},
		{"/v3/", 404},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != tt.want {
			t.Fatalf("%s: expected %d, got %d", tt.path, tt.want, rec.Code)
		}
	}
}

func TestExactMatch(t *testing.T) {
	r := New()
	r.HandleFunc("GET /foo/{$}", Allow, handler200)

	tests := []struct {
		path string
		want int
	}{
		{"/foo/", 200},
		{"/foo/bar", 404},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != tt.want {
			t.Fatalf("%s: expected %d, got %d", tt.path, tt.want, rec.Code)
		}
	}
}

func TestWildcard(t *testing.T) {
	r := New()
	var got string
	r.HandleFunc("GET /sites/{project}/branch/{branch}/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		got = req.PathValue("path")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/sites/myapp/branch/main/css/style.css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "css/style.css" {
		t.Fatalf("expected path=%q, got %q", "css/style.css", got)
	}
}

func TestWildcardEmpty(t *testing.T) {
	r := New()
	var got string
	r.HandleFunc("GET /files/{path...}", Allow, func(w http.ResponseWriter, req *http.Request) {
		got = req.PathValue("path")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/files/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "" {
		t.Fatalf("expected path=%q, got %q", "", got)
	}
}

func TestPriorityLiteralBeatsParam(t *testing.T) {
	r := New()
	var which string
	r.HandleFunc("GET /api/v1/projects", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "literal"
		w.WriteHeader(200)
	})
	r.HandleFunc("GET /{rest...}", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "wildcard"
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if which != "literal" {
		t.Fatalf("expected literal route, got %q", which)
	}
}

func TestAuthRequired(t *testing.T) {
	r := New()

	deny := &denyAuth{}
	r.Register(deny)

	r.HandleFunc("GET /secret", deny, handler200)
	r.HandleFunc("GET /public", Allow, handler200)

	req := httptest.NewRequest("GET", "/secret", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "/public", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

type denyAuth struct{}

func (denyAuth) Authorize(w http.ResponseWriter, _ *http.Request) bool {
	http.Error(w, "Forbidden", http.StatusForbidden)
	return false
}

func TestAuthUnregisteredPanics(t *testing.T) {
	r := New()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for unregistered auth")
		}
	}()

	r.HandleFunc("GET /foo", &denyAuth{}, handler200)
}

func TestAuthNilPanics(t *testing.T) {
	r := New()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil auth")
		}
	}()

	r.HandleFunc("GET /foo", nil, handler200)
}

func TestQueryParamExtraction(t *testing.T) {
	r := New()
	var id, fmt string
	r.HandleFunc("GET /static?id={id}&fmt={fmt}", Allow, func(w http.ResponseWriter, req *http.Request) {
		id = req.PathValue("id")
		fmt = req.PathValue("fmt")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/static?id=abc&fmt=raw", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if id != "abc" {
		t.Fatalf("expected id=%q, got %q", "abc", id)
	}
	if fmt != "raw" {
		t.Fatalf("expected fmt=%q, got %q", "raw", fmt)
	}
}

func TestQueryParamMissing(t *testing.T) {
	r := New()
	var id string
	r.HandleFunc("GET /static?id={id}", Allow, func(w http.ResponseWriter, req *http.Request) {
		id = req.PathValue("id")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/static", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if id != "" {
		t.Fatalf("expected id=%q, got %q", "", id)
	}
}

func TestAPTMultiSegmentProject(t *testing.T) {
	r := New()
	var project string
	r.HandleFunc("GET /apt/{project}/dists/stable/InRelease", Allow, func(w http.ResponseWriter, req *http.Request) {
		project = req.PathValue("project")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/apt/my-org/my-project/dists/stable/InRelease", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if project != "my-org/my-project" {
		t.Fatalf("expected project=%q, got %q", "my-org/my-project", project)
	}
}

func TestMixedSegmentBinaryArch(t *testing.T) {
	r := New()
	var project, arch string
	r.HandleFunc("GET /apt/{project}/dists/stable/main/binary-{arch}/Packages", Allow, func(w http.ResponseWriter, req *http.Request) {
		project = req.PathValue("project")
		arch = req.PathValue("arch")
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/apt/my-org/my-project/dists/stable/main/binary-amd64/Packages", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if project != "my-org/my-project" {
		t.Fatalf("expected project=%q, got %q", "my-org/my-project", project)
	}
	if arch != "amd64" {
		t.Fatalf("expected arch=%q, got %q", "amd64", arch)
	}
}

func TestMultiMethodSamePath(t *testing.T) {
	r := New()
	var method string
	r.HandleFunc("GET /sites/{project}/branch/{branch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		method = "GET"
		w.WriteHeader(200)
	})
	r.HandleFunc("PUT /sites/{project}/branch/{branch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		method = "PUT"
		w.WriteHeader(200)
	})
	r.HandleFunc("DELETE /sites/{project}/branch/{branch}", Allow, func(w http.ResponseWriter, req *http.Request) {
		method = "DELETE"
		w.WriteHeader(200)
	})

	for _, m := range []string{"GET", "PUT", "DELETE"} {
		method = ""
		req := httptest.NewRequest(m, "/sites/myapp/branch/main", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: expected 200, got %d", m, rec.Code)
		}
		if method != m {
			t.Fatalf("expected method=%q, got %q", m, method)
		}
	}

	req := httptest.NewRequest("POST", "/sites/myapp/branch/main", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Fatalf("POST: expected 405, got %d", rec.Code)
	}
}
