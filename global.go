package httx

import "net/http"

var DefaultServeMux = NewMux()

func Handle(method, path string, handler HandlerFunc) {
	DefaultServeMux.Handle(method, path, handler)
}

func GET(path string, handler HandlerFunc) {
	DefaultServeMux.GET(path, handler)
}

func POST(path string, handler HandlerFunc) {
	DefaultServeMux.POST(path, handler)
}

func PUT(path string, handler HandlerFunc) {
	DefaultServeMux.PUT(path, handler)
}

func PATCH(path string, handler HandlerFunc) {
	DefaultServeMux.PATCH(path, handler)
}

func DELETE(path string, handler HandlerFunc) {
	DefaultServeMux.DELETE(path, handler)
}

func HEAD(path string, handler HandlerFunc) {
	DefaultServeMux.HEAD(path, handler)
}

func CONNECT(path string, handler HandlerFunc) {
	DefaultServeMux.CONNECT(path, handler)
}

func OPTIONS(path string, handler HandlerFunc) {
	DefaultServeMux.OPTIONS(path, handler)
}

func TRACE(path string, handler HandlerFunc) {
	DefaultServeMux.TRACE(path, handler)
}

func ANY(path string, handler HandlerFunc) {
	DefaultServeMux.ANY(path, handler)
}

func Merge(path string, handler http.Handler) {
	DefaultServeMux.Merge(path, handler)
}
