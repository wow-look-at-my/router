# router

HTTP request router with multi-segment path parameters.

## Build

```bash
go-toolchain
```

## Key design

- `{param}` matches 1+ path segments using greedy backtracking. `/dl/{project}/latest/{os}/{arch}` matches `/dl/my-org/my-project/latest/linux/amd64` with `project=my-org/my-project`.
- `{param...}` is a wildcard matching 0+ remaining segments. Must be last.
- Mixed segments: `{project}.rb`, `binary-{arch}`, `{project}-{os}-{arch}`.
- Optional host portion before the path: `apt.{domain}/{path...}` or `{project}.pazer.site/{path...}` matches by `req.Host`. Literal labels match exactly; a whole-label `{name}` in a non-final position matches exactly ONE label; a trailing `{name}` host wildcard binds the remaining host labels. Both bind via `req.PathValue(name)`; partial-label host params (`foo{x}bar`) are rejected. Literal host label count outranks every path term in `priority()` (identical host portions stay neutral, so path scoring is untouched within a family). Host-agnostic patterns (starting with `/`) match any host. Strict partitioning: a request host that matches any host-bearing route excludes host-agnostic routes (a known subdomain never falls through to bare-host routes). Pure path routers are unaffected (zero overhead).
- `Routes()` returns consolidated route list with methods grouped by path.
- Per-route `Auth` interface. `router.Allow` for public routes. Auth must be `Register()`'d first.
- `Tracer`/`Span` interfaces for OTEL integration without hard dependency.
- Pattern syntax matches Go 1.22+ ServeMux: `METHOD /path/{param}`, `/prefix/`, `{$}`, `{rest...}`.
- Best-match routing (most literals wins), not first-match.
- 405 with Allow header when path matches but method doesn't.

## File layout

- `router.go` -- Router type, Handle, HandleFunc, Routes, ServeHTTP (host partitioning + path matching), Auth
- `pattern.go` -- Pattern parsing (host portion + path), `full()` host+path accessor
- `match.go` -- Recursive backtracking path matcher, host label matcher (`matchHostSegs`/`splitHost`)
- `otel.go` -- Tracer/Span interfaces, statusWriter
