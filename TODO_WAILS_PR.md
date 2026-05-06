# TODO: Open upstream Wails3 PR for the IPv4/IPv6 dev-server bug

## Reminder

File a PR against [`wailsapp/wails`](https://github.com/wailsapp/wails) to fix the dev-server proxy regression introduced in alpha.79+. Currently worked around locally with `server.host: "127.0.0.1"` in `echo/frontend/vite.config.ts`.

## Context

- **Issue:** [#5348](https://github.com/wailsapp/wails/issues/5348) — alpha.85 (also alpha.79–.84) `wails3 init` apps fail to load on macOS / some Windows setups. Window shows network error or blank.
- **Related:** [#5059](https://github.com/wailsapp/wails/issues/5059), [#4556](https://github.com/wailsapp/wails/issues/4556)
- **Regression source:** PR [#5265](https://github.com/wailsapp/wails/pull/5265) — added `retryTransport` with unconditional `tcp4` for localhost in `v3/internal/assetserver/build_dev.go`. Comment says "to avoid IPv6 issues on Windows" but it breaks every system where the Vite dev server binds IPv6-only (Node 18+ default on macOS).

## Symptom

```
ERR [ExternalAssetHandler] Proxy error error="dial tcp4 127.0.0.1:9245: connect: connection refused"
```

Even though `curl http://localhost:9245` works fine — Vite is listening on `[::1]:9245`, the wails proxy is forcing `tcp4 127.0.0.1`.

Confirm with: `lsof -iTCP:9245 -sTCP:LISTEN -P -n`

## Fix

File: `v3/internal/assetserver/build_dev.go`

Replace the unconditional `tcp4` force with a dual-stack fallback (try IPv4 then IPv6 for any localhost-ish hostname). See the chat history / draft I wrote — uses an `isLocalhost` helper plus a `dialLocalhostDualStack` function that returns the first stack that connects, and `errors.Join`s both errors if neither does.

## PR checklist

- [ ] Fork `wailsapp/wails`, branch `fix/issue-5348-dual-stack-localhost` off `master`
- [ ] Apply the dual-stack `DialContext` change in `v3/internal/assetserver/build_dev.go`
- [ ] Test on macOS (Vite IPv6-only — current broken case)
- [ ] Test on Windows (original `tcp4` case from PR #5265 — must still work)
- [ ] Reference #5348, #5059, #4556, and PR #5265 in the PR description
- [ ] Note that the local Vite `host: "127.0.0.1"` workaround can be removed once a Wails alpha containing the fix ships

## Local workaround (already applied)

`echo/frontend/vite.config.ts`:

```ts
server: {
  host: "127.0.0.1",
}
```

Remove this once the upstream fix is released.
