# AGENTS.md

## Layout

- `main.go` only calls `app.Run`.
- `internal/app`: the window, views and the app's state, including what it does with realtime events. Its embedded `icons/`, `fonts/` and `themes/` live beside it.
- `internal/cohub`: the Cohub API's types and client, the gateway's WebSocket, and the reducer of streamed replies. No views and no copy: text for the window, like an untitled chat's name, stays in `internal/app`.
- `internal/i18n`: the copy in zh-CN and en. `messages/` is the web app's catalogs, word for word; the app's own words go in `extraMessages`. Views call `m("key")`.
- `internal/cron`: parsing cron expressions and previewing their runs.

Put new logic that needs no view or `*app` into a package of its own under `internal/`, and keep `internal/app` for what draws or holds the app's state. Follow `.agents/skills/mygo-maintenance` for MyGo's UI rules.

## Tests

Only write a test for behaviour that can actually break. If the code under test could change without the test noticing, or the test would fail on a harmless rewording, don't write it.

**Write tests for:**

- What input does: clicks, keys, drops and menus, and the state or effect they lead to (Enter sends, ⌘K opens a Space, archiving from a row's menu, dropping files on the composer).
- Logic that doesn't need a view: routes and `cohub://` links, the patch reducer, cron expressions.
- A regression you just fixed. Say in the test's comment what went wrong.

**Don't write tests that:**

- Check that a list of labels shows on screen. That repeats the view code and breaks whenever the wording changes.
- Check default settings or the order of rows in a form.
- Test MyGo itself.
- Depend on the wall clock. Use `a.now`, which `demoApp` fixes.

**How:**

- Build views with `ui.NewTester` on `demoApp(t)` (in `internal/app/main_test.go`), which loads the demo data in zh-CN. Find elements by their text or accessibility label, then click, type or press keys, and check the app's state afterwards.
- Look at visuals instead of asserting on them. `snapshot` writes PNGs to `$SNAPSHOT_DIR` when it's set; give it an absolute path, since tests run in their package's directory.
- Tests that need the signed-in account skip unless `COHUB_LIVE` (or `COHUB_LIVE_STREAM`) is set. `TestCompare` puts the app's renders beside the web's in `compare/`.
- Start each test with a one-sentence comment saying the behaviour it checks.
- Before committing, run `gofmt -l .`, `go tool mygo vet .` and `CGO_ENABLED=0 go test ./...`, and run a new UI test with `-count=5`. Fix or delete a flaky test; don't retry it.

## Releasing

See [RELEASING.md](RELEASING.md).
