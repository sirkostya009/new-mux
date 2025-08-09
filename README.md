# HTTp eXtended

Fork of [fasthttp/router](https://github.com/fasthttp/router/) adapted for `http.Handler`.

Engineered as a simple improvement upon standard net/http implementation of ServeMux, with main algorithm and ergonomics largely borrowed from fasthttp/router.

Thus, this multiplexer has optional and regex path params unlike the standard mux.

Groups are also supported, but their ergonomics aren't traditional, instead you simply merge different handlers with Mux.Merge.

Inherits 0 allocation routing, except for redirects. This is a deliberate choice attempting to strip away any external deps from codebase.

Additionally, RedirectResolvedPath (RedirectFixedPath in `fasthttp/router`) works differently by utilizing url.ResolveReference method.

You _may_ want to disable redirects if you run into GC issues (but this router would probably be the least of your allocation problems anyway).

# Usage

```go
mux := httx.NewMux()

mux.OnError = func(w http.ResponseWriter, r *http.Request, err error) {
	// handle err
}

// Middleware must be initialized before any route
mux.Pre(func(next httx.HandlerFunc) httx.HandlerFunc {
	return func (w http.ResponseWriter, r *http.Request) (err error) {
		start := time.Now()
		defer func() { // must defer stuff running after because panics
			finish := time.Now()
			slog.Info("request", "method", r.Method, "uri", r.RequestURI, "time-ms", finish.Sub(start).Milliseconds())
		}()
		return next(w, r)
	}
})

// Method prefix is available since go ver 1.22
mux.GET("/hello", func(w http.ResponseWriter, r *http.Request) error {
	_, err := w.Write([]byte("world!"))
	return err
})

mux.GET(`/{id:\d+}`, func(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id") // Go's 1.22 PathValue-compatible
	res := someDatabaseFunc(r.Context())
	return json.NewEncoder(w).Encode(res)
})

_ = http.ListenAndServe(":8080", mux)
```

## TODO:

- [ ] Fix all tests
- [ ] Add `FS` method

## License

The original BSD 3-clause license from [fasthttp/router](https://github.com/fasthttp/router/blob/master/LICENSE). See [LICENSE](LICENSE).
