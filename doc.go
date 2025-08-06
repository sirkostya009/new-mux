/*
A simple improvement upon standard net/http implementation of ServeMux, forked from fasthttp/router.

Thus, this multiplexer introduces optional and regex path params.

Grouping is also supported, but their ergonomics aren't traditional, instead you simply merge different handlers with Mux.Merge.

# Usage

	mux := httx.NewMux()

	mux.OnError = func(w http.ResponseWriter, r *http.Request, err error) {
		// handle err
	}

	// Middleware must be initialized before any route
	mux.Pre(func(next httx.HandlerFunc) httx.HandlerFunc {
		return func (w http.ResponseWriter, r *http.Request) (err error) {
			start := time.Now()
			err = next(w, r)
			finish := time.Now()
			slog.Info("request", "duration", finish.Sub(start))
			return
		}
	})

	// Method prefix is available since go ver 1.22
	mux.GET("/hello", func (w http.ResponseWriter, r *http.Request) error {
		_, err := w.Write([]byte("world!"))
		return err
	})

	mux.GET(`/{id:\d+}`, func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id") // Go's 1.22 PathValue-compatible
		res := someDatabaseFunc(r.Context())
		return json.NewEncoder(w).Encode(res)
	})

	_ = http.ListenAndServe(":8080", mux)
*/
package httx
