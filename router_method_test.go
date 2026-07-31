package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test405WithAllow(t *testing.T) {
	r := New()
	r.HandleFunc("GET /resource", Allow, handler200)
	r.HandleFunc("PUT /resource", Allow, handler200)

	req := httptest.NewRequest("DELETE", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, 405, rec.Code)

	allow := rec.Header().Get("Allow")
	require.Equal(t, "GET, HEAD, OPTIONS, PUT", allow)

}

func TestHEADUsesGETRoute(t *testing.T) {
	r := New()
	var gotMethod, gotID string
	r.HandleFunc("GET /resource/{id}", Allow, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotID = req.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest("HEAD", "/resource/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "HEAD", gotMethod)
	require.Equal(t, "abc", gotID)
}

func TestExplicitHEADRouteBeatsGETFallback(t *testing.T) {
	r := New()
	var which string
	r.HandleFunc("GET /resource", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "GET"
		w.WriteHeader(http.StatusOK)
	})
	r.HandleFunc("HEAD /resource", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "HEAD"
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest("HEAD", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "HEAD", which)
}

func Test405AllowIncludesImplicitHEADForGET(t *testing.T) {
	r := New()
	r.HandleFunc("GET /resource", Allow, handler200)
	r.HandleFunc("PUT /resource", Allow, handler200)

	req := httptest.NewRequest("POST", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, "GET, HEAD, OPTIONS, PUT", rec.Header().Get("Allow"))
}

func TestOPTIONSReturnsAllowForKnownPath(t *testing.T) {
	r := New()
	called := false
	r.HandleFunc("GET /resource", Allow, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	r.HandleFunc("PUT /resource", Allow, handler200)

	req := httptest.NewRequest("OPTIONS", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "GET, HEAD, OPTIONS, PUT", rec.Header().Get("Allow"))
	require.False(t, called)
}

func TestExplicitOPTIONSRouteBeatsAutomatic(t *testing.T) {
	r := New()
	var which string
	r.HandleFunc("GET /resource", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "GET"
		w.WriteHeader(http.StatusOK)
	})
	r.HandleFunc("OPTIONS /resource", Allow, func(w http.ResponseWriter, _ *http.Request) {
		which = "OPTIONS"
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest("OPTIONS", "/resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "OPTIONS", which)
}
