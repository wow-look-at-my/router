# router

HTTP request router with multi-segment path parameters.

## Features

- **Slashes in path values**: `{param}` matches across `/` boundaries using greedy backtracking. No `%2F` encoding needed.
- **Route introspection**: `Routes()` returns all registered routes with methods consolidated by path.
- **Correct 405**: Returns 405 Method Not Allowed with `Allow` header instead of 404.
- **Automatic HEAD/OPTIONS**: `HEAD` requests route to matching `GET` handlers, and `OPTIONS` returns `Allow`, unless explicit handlers are registered.
- **Per-route auth**: Registered `Auth` interface checked before handler runs. `router.Allow` for public routes.
- **Tracing**: Optional `Tracer` interface for OpenTelemetry integration with zero hard dependencies.
- **Host matching**: A pattern may carry a host portion before the path (`apt.{domain}/{path...}`, `{project}.pazer.site/{path...}`). A non-final `{name}` label matches exactly one label; a final `{name}` captures the trailing labels.
- **Pattern syntax**: Compatible with Go 1.22+ ServeMux patterns, plus intra-segment params and query param extraction.

## Usage

```go
r := router.New()

// Per-route auth
r.Register(myAuth)
r.HandleFunc("GET /api/v1/projects/{project}", myAuth, projectHandler)
r.HandleFunc("GET /healthz", router.Allow, healthHandler)

// Multi-segment params work naturally
// GET /dl/my-org/my-project/latest/linux/amd64
// -> project = "my-org/my-project"
r.HandleFunc("GET /dl/{project}/latest/{os}/{arch}", myAuth, dlHandler)

// Intra-segment patterns
r.HandleFunc("GET /brew/{project}.rb", myAuth, brewHandler)

// Prefix matching
r.HandleFunc("/v2/", myAuth, ociHandler)

// Wildcard
r.HandleFunc("GET /sites/{project}/branch/{branch}/{path...}", myAuth, siteHandler)

// Host matching: serve a subdomain and capture the base domain.
// GET apt.example.com/dists/stable/InRelease
// -> domain = "example.com", path = "dists/stable/InRelease"
r.HandleFunc("GET apt.{domain}/{path...}", myAuth, aptHandler)

// Non-final host params match exactly one label.
// GET tesla-wheel-data.pazer.site/index.html -> project = "tesla-wheel-data"
r.HandleFunc("GET {project}.pazer.site/{path...}", myAuth, projectSiteHandler)

// Route listing
for _, route := range r.Routes() {
    fmt.Println(route) // "apt.{domain}/{path...} {GET}"
}

http.ListenAndServe(":8080", r)
```

## Host matching

A pattern that does not begin with `/` (after the optional method) carries a
**host portion** before the path, separated by the first `/`:

```
GET apt.{domain}/{path...}
    └── host ──┘└── path ──┘
```

- Literal host labels (`apt`) must match the corresponding labels of
  `req.Host` exactly (the port is stripped).
- A whole-label `{name}` in a **non-final** position matches exactly one label
  and binds it: `{project}.pazer.site` + `Host: myapp.pazer.site` gives
  `project = "myapp"`, available via `req.PathValue("project")`.
- A **final** `{name}` label is a host wildcard binding the remaining labels
  (`apt.foo.example.com` -> `domain = "foo.example.com"`), available via
  `req.PathValue("domain")`. The forms combine: `{sub}.{domain}` is valid.
- Partial-label params (`foo{x}bar`), the empty `{}`, and a host with no
  following path all panic in `Handle` — host params are whole labels only.
- A pattern with **no** host portion is host-agnostic and matches any host.

Between host-bearing patterns matching the same request, the one with **more
literal host labels** wins outright: `{project}.pazer.site/{path...}` (2 host
literals) beats `dl.{domain}/{path...}` (1) for every path on `*.pazer.site`,
so each host family claims its hosts coherently. Patterns with identical host
portions keep the ordinary path-based priority between them.

Host partitioning is strict: when a request host matches at least one
host-bearing route, host-agnostic routes are **not** eligible for that request.
A known subdomain therefore commits to its own routes and never falls through to
bare-host routes (an unmatched path under that subdomain is a 404). Requests
whose host matches no host-bearing route are served by host-agnostic routes as
usual. Routers that register no host portion pay zero overhead for this.

## Matching algorithm

Parameters try the maximum number of segments first, backtracking until the rest of the pattern matches. When a wildcard (`{param...}`) follows, parameters use minimum-first so the wildcard captures remaining segments.

Priority is best-match: literal host labels rank above all path terms, then more literal segments wins over fewer, exact matches beat prefix matches, non-wildcard beats wildcard.
