# Streamplace Go conventions

How we write Go in Streamplace. The linter may not catch these conventions, but
we expect you to follow them in this repo.

## Logging

Log through `stream.place/streamplace/pkg/log`. Each call takes the context
first, then the message, then alternating key-value pairs:

```go
log.Log(ctx, "started ingest", "streamer", did, "rtmpPort", port)
```

Pick the level by what happened. The number is the glog `-v` level that a process
needs to print the line. The default is `-v=3`. At the default, `Error`, `Warn`,
and `Log` always print. `Debug` and `Trace` print only at a higher `-v`.

| Call        | min `-v` | Use it for                                   |
| ----------- | -------- | -------------------------------------------- |
| `log.Error` | 1        | An operation failed and could not complete.  |
| `log.Warn`  | 2        | Something was wrong, but the code recovered. |
| `log.Log`   | 3        | General information. This is the info level. |
| `log.Debug` | 4        | Detail for debugging. Prints only at `-v=4`. |
| `log.Trace` | 9        | Detailed tracing. Prints only at `-v=9`.     |

There is no `log.Info`. `log.Log` is the info level.

Common mistakes:

- The key-value arguments come in pairs. An odd number of arguments prints a
  warning at runtime and may corrupt the line. Always check the arguments after you edit a
  call.
- Do not pass a `caller` key. The logger adds one for you.

A value such as a request id or a streamer did can apply to every log line from a
context. To attach such a value, derive a new context with `log.WithLogValues`.
You do not need to pass the value to every call:

```go
ctx = log.WithLogValues(ctx, "streamer", did)
```

## Error wrapping

Wrap errors with `%w`, not `%v`, so callers can inspect the cause with
`errors.Is` and `errors.As`:

```go
return fmt.Errorf("failed to combine segments: %w", err)
```

`%v` converts the error to a string and breaks the error chain. A caller can no
longer match the underlying error. Use `%v` only when you want to hide the cause.

## HTTP handlers

Write error responses with the helpers in `stream.place/streamplace/pkg/errors`.
Do not call `w.WriteHeader` and encode JSON yourself, as the function will do it for you:

```go
if err != nil {
    errors.WriteHTTPBadRequest(w, "Error parsing PUSH_REWRITE payload", err)
    return
}
```

| Helper                          | Status | Notes                   |
| ------------------------------- | ------ | ----------------------- |
| `WriteHTTPBadRequest`           | 400    |                         |
| `WriteHTTPUnauthorized`         | 401    |                         |
| `WriteHTTPForbidden`            | 403    |                         |
| `WriteHTTPNotFound`             | 404    |                         |
| `WriteHTTPUnsupportedMediaType` | 415    |                         |
| `WriteHTTPTooManyRequests`      | 429    | takes no `err` argument |
| `WriteHTTPInternalServerError`  | 500    |                         |
| `WriteHTTPNotImplemented`       | 501    |                         |

Each helper returns an `APIError{Msg, Status, Err}`. Most handlers ignore it and
return after they write the response. Use the return value only when the caller
needs the status or the message.

## Tests

Assert with `github.com/stretchr/testify/require`, not `assert`. `require` fails
the test immediately (`t.FailNow`) on the first broken precondition. A failed
setup then stops at once, instead of causing nil-pointer panics in the assertions
that follow:

```go
require.NoError(t, err)
require.Equal(t, "sync", cmd.Name)
```

Media tests using `withNoGSTLeaks` must stop their pipelines and cancel their
contexts before returning. The leak helper waits for finalizer passes before
requesting the native snapshot; keep that barrier synchronous and never block the
finalizer queue, which also releases Go-owned GStreamer references. Verify changes
to the helper with race-enabled media tests without disabling leak assertions.

## GStreamer bindings

Use the generated `go-gst/pkg/gst` and `go-gst/pkg/gstapp` APIs. GObjects such as
elements, pads, bins, and pipelines are interfaces. Check boolean link/add results
and `StateChangeReturn` values rather than treating them as Go errors.

GObject properties require fixed-width numeric values: `int32` for `gint`,
`uint32` for `guint`, and `uint64` for `guint64`. The new GValue conversion rejects
Go `int` and `uint`.

`MapInfo.Data()` is a view of native memory, valid only until `MapInfo.Unmap()`.
Synchronous writers may consume it directly. Use `copyBufferData` when bytes
escape into queues or caches; Pion's H264 payloader can retain SPS/PPS slices
between calls. Use `newBufferFromBytes` for writable, Go-sourced payloads.

Sample buffers and caps are borrowed getters that retain their sample through
the binding's GC ownership mechanism. Do not explicitly unref a sample while its
borrowed children remain in use. The bindings now handle sample finalization;
callers must not install their own finalizers or repeat the old sample workaround.

The published dependency pins include the opaque-owner tiny-allocation fix:
[go-glib #37](https://github.com/go-gst/go-glib/pull/37) and
[go-gst #228](https://github.com/go-gst/go-gst/pull/228). Remove the remote fork
replacements when an upstream release contains both fixes.
