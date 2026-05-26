# router

HTTP request router with multi-segment path parameters.

## Features

- **Slashes in path values**: `{param}` matches across `/` boundaries using greedy backtracking. No `%2F` encoding needed.
- **Route introspection**: `Routes()` returns all registered routes with methods consolidated by path.
- **Correct 405**: Returns 405 Method Not Allowed with `Allow` header instead of 404.
- **Per-route auth**: Registered `Auth` interface checked before handler runs. `router.Allow` for public routes.
- **Tracing**: Optional `Tracer` interface for OpenTelemetry integration with zero hard dependencies.
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

// Route listing
for _, route := range r.Routes() {
    fmt.Println(route) // "/dl/{project}/latest/{os}/{arch} {GET}"
}

http.ListenAndServe(":8080", r)
```

## Matching algorithm

Parameters try the maximum number of segments first, backtracking until the rest of the pattern matches. When a wildcard (`{param...}`) follows, parameters use minimum-first so the wildcard captures remaining segments.

Priority is best-match: more literal segments wins over fewer, exact matches beat prefix matches, non-wildcard beats wildcard.
