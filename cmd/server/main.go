package main

import (
	"log/slog"
	"net/http"

	"github.com/sirkostya009/httx"
)

func main() {
	httx.GET("/super-duper-func", func(w http.ResponseWriter, r *http.Request) error {
		slog.Info("incoming", slog.String("method", r.Method), slog.String("url", r.URL.String()))
		_, err := w.Write([]byte("message"))
		return err
	})

	httx.Merge("/fs/*", http.FileServer(http.Dir("./radix")))

	http.ListenAndServe(":8080", httx.DefaultServeMux)
}
