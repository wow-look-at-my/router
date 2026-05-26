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
- `Routes()` returns consolidated route list with methods grouped by path.
- Per-route `Auth` interface. `router.Allow` for public routes. Auth must be `Register()`'d first.
- `Tracer`/`Span` interfaces for OTEL integration without hard dependency.
- Pattern syntax matches Go 1.22+ ServeMux: `METHOD /path/{param}`, `/prefix/`, `{$}`, `{rest...}`.
- Best-match routing (most literals wins), not first-match.
- 405 with Allow header when path matches but method doesn't.

## File layout

- `router.go` -- Router type, Handle, HandleFunc, Routes, ServeHTTP, Auth
- `pattern.go` -- Pattern parsing
- `match.go` -- Recursive backtracking matcher
- `otel.go` -- Tracer/Span interfaces, statusWriter
