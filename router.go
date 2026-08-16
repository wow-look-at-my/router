package router

import (
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
)

// Auth is the interface for per-route authorization.
// Global authentication (token extraction, OIDC, etc.) belongs in
// application middleware outside the router.
type Auth interface {
	Authorize(w http.ResponseWriter, r *http.Request) bool
}

type allowAuth struct{}

func (allowAuth) Authorize(http.ResponseWriter, *http.Request) bool { return true }

// Allow is the Auth for public routes. Always authorizes.
// Pre-registered on every Router.
var Allow Auth = &allowAuth{}

// Option configures a Router.
type Option func(*Router)

// Router is an HTTP request router with support for multi-segment path
// parameters, per-route auth, and OpenTelemetry tracing.
type Router struct {
	mu            sync.RWMutex
	routes        []registeredRoute
	auths         set.Set[Auth]
	tracer        Tracer
	hasHostRoutes bool // any registered route carries a host portion
}

type registeredRoute struct {
	pat     *pattern
	auth    Auth
	handler http.Handler
}

// Route describes a registered path and the HTTP methods bound to it.
type Route struct {
	Pattern string
	Methods []string // nil means any method
}

// String formats the route as "/path {GET,POST}" or "/path {*}" for any-method.
func (r Route) String() string {
	if r.Methods == nil {
		return r.Pattern + " {*}"
	}
	return r.Pattern + " {" + strings.Join(r.Methods, ",") + "}"
}

// New creates a Router. router.Allow is pre-registered.
func New(opts ...Option) *Router {
	r := &Router{
		auths: set.Of[Auth](Allow),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Register adds an Auth implementation. Routes can only reference Auth values
// that have been registered. Panics on nil or duplicate (pointer identity).
func (r *Router) Register(auth Auth) {
	if auth == nil {
		panic("router: cannot register nil Auth")
	}
	if !r.auths.Add(auth) {
		panic("router: duplicate Auth registration")
	}
}

// Handle registers a route with the given auth and handler.
// The auth must have been Register()'d first (or be router.Allow).
// Panics on nil auth, unregistered auth, or invalid pattern.
func (r *Router) Handle(pattern string, auth Auth, handler http.Handler) {
	if auth == nil {
		panic("router: auth must not be nil")
	}
	if !r.auths.Contains(auth) {
		panic("router: auth not registered (call Register first)")
	}
	pat, err := parsePattern(pattern)
	if err != nil {
		panic("router: " + err.Error())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, registeredRoute{
		pat:     pat,
		auth:    auth,
		handler: handler,
	})
	if len(pat.hostSegs) > 0 {
		r.hasHostRoutes = true
	}
}

// HandleFunc registers a route with a handler function.
func (r *Router) HandleFunc(pattern string, auth Auth, handler http.HandlerFunc) {
	r.Handle(pattern, auth, handler)
}

// Routes returns all registered routes, consolidated by path pattern.
// Multiple registrations for the same path with different methods produce
// a single Route with all methods listed (sorted).
func (r *Router) Routes() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type entry struct {
		route Route
		order int
	}
	groups := make(map[string]*entry)
	idx := 0

	for _, rr := range r.routes {
		key := rr.pat.full()
		if e, ok := groups[key]; ok {
			if rr.pat.method != "" && !slices.Contains(e.route.Methods, rr.pat.method) {
				e.route.Methods = append(e.route.Methods, rr.pat.method)
			}
		} else {
			e := &entry{route: Route{Pattern: key}, order: idx}
			idx++
			if rr.pat.method != "" {
				e.route.Methods = []string{rr.pat.method}
			}
			groups[key] = e
		}
	}

	result := make([]Route, 0, len(groups))
	for _, e := range groups {
		if e.route.Methods != nil {
			slices.Sort(e.route.Methods)
		}
		result = append(result, e.route)
	}
	slices.SortFunc(result, func(a, b Route) int {
		return strings.Compare(a.Pattern, b.Pattern)
	})
	return result
}

// ServeHTTP dispatches the request to the best matching route.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rawPath := req.URL.RawPath
	if rawPath == "" {
		rawPath = req.URL.Path
	}
	pathSegs := splitPath(rawPath)
	trailing := hasTrailingSlash(rawPath)

	r.mu.RLock()
	routes := r.routes
	hasHost := r.hasHostRoutes
	r.mu.RUnlock()

	// Host partitioning: when the request host matches any host-bearing route,
	// it is "claimed" and host-agnostic routes become ineligible. This makes a
	// known subdomain commit to its own routes (never falling through to bare-
	// host routes), exactly as a per-subdomain dispatch table would. Pure path
	// routers register no host routes and skip this entirely.
	var hostLabels []string
	hostClaimed := false
	if hasHost {
		hostLabels = splitHost(req.Host)
		for i := range routes {
			if len(routes[i].pat.hostSegs) == 0 {
				continue
			}
			if _, ok := matchHostSegs(routes[i].pat.hostSegs, hostLabels); ok {
				hostClaimed = true
				break
			}
		}
	}

	var best *registeredRoute
	var bestParams map[string]string
	var bestScore int
	var allowed []string

	for i := range routes {
		rr := &routes[i]
		var hostValues []string
		if len(rr.pat.hostSegs) > 0 {
			v, ok := matchHostSegs(rr.pat.hostSegs, hostLabels)
			if !ok {
				continue
			}
			hostValues = v
		} else if hostClaimed {
			continue
		}
		params, ok := tryMatch(rr.pat, pathSegs, trailing)
		if !ok {
			continue
		}
		for j, name := range rr.pat.hostParams {
			params[name] = hostValues[j]
		}
		methodScore, ok := matchMethod(rr.pat.method, req.Method)
		if !ok {
			allowed = appendAllowedMethods(allowed, rr.pat.method)
			continue
		}
		score := rr.pat.priority()*10 + methodScore
		if best == nil || score > bestScore {
			best = rr
			bestParams = params
			bestScore = score
		}
	}

	if best == nil {
		if len(allowed) > 0 {
			allowed = append(allowed, http.MethodOptions)
			slices.Sort(allowed)
			allowed = slices.Compact(allowed)
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, req)
		return
	}

	for name, value := range bestParams {
		req.SetPathValue(name, value)
	}

	if len(best.pat.queryParams) > 0 {
		q := req.URL.Query()
		for _, qp := range best.pat.queryParams {
			if v := q.Get(qp.key); v != "" {
				req.SetPathValue(qp.param, v)
			}
		}
	}

	if !best.auth.Authorize(w, req) {
		return
	}

	if r.tracer != nil {
		var span Span
		ctx, span := r.tracer.Start(req.Context(), req.Method+" "+best.pat.path)
		defer span.End()
		req = req.WithContext(ctx)
	}

	best.handler.ServeHTTP(w, req)
}

func matchMethod(routeMethod, requestMethod string) (int, bool) {
	if routeMethod == "" {
		return 0, true
	}
	if routeMethod == requestMethod {
		return 2, true
	}
	if routeMethod == http.MethodGet && requestMethod == http.MethodHead {
		return 1, true
	}
	return 0, false
}

func appendAllowedMethods(allowed []string, routeMethod string) []string {
	if routeMethod == "" {
		return allowed
	}
	allowed = append(allowed, routeMethod)
	if routeMethod == http.MethodGet {
		allowed = append(allowed, http.MethodHead)
	}
	return allowed
}
