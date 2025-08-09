package httx

import (
	"net/http"
	"strings"
)

type Group struct {
	prefix string
	m      *Mux
}

func (g *Group) Group(prefix string) *Group {
	if !strings.HasPrefix(prefix, "/") {
		panic(`group prefix must begin with "/"`)
	}
	return &Group{g.prefix + prefix, g.m}
}

func (g *Group) Handle(method, path string, handler HandlerFunc) {
	g.m.Handle(method, g.prefix+path, handler)
}

func (g *Group) GET(path string, handler HandlerFunc) {
	g.m.GET(g.prefix+path, handler)
}

func (g *Group) POST(path string, handler HandlerFunc) {
	g.m.POST(g.prefix+path, handler)
}

func (g *Group) PUT(path string, handler HandlerFunc) {
	g.m.PUT(g.prefix+path, handler)
}

func (g *Group) PATCH(path string, handler HandlerFunc) {
	g.m.PATCH(g.prefix+path, handler)
}

func (g *Group) DELETE(path string, handler HandlerFunc) {
	g.m.DELETE(g.prefix+path, handler)
}

func (g *Group) HEAD(path string, handler HandlerFunc) {
	g.m.HEAD(g.prefix+path, handler)
}

func (g *Group) CONNECT(path string, handler HandlerFunc) {
	g.m.CONNECT(g.prefix+path, handler)
}

func (g *Group) OPTIONS(path string, handler HandlerFunc) {
	g.m.OPTIONS(g.prefix+path, handler)
}

func (g *Group) TRACE(path string, handler HandlerFunc) {
	g.m.TRACE(g.prefix+path, handler)
}

func (g *Group) ANY(path string, handler HandlerFunc) {
	g.m.ANY(g.prefix+path, handler)
}

func (g *Group) Merge(path string, handler http.Handler) {
	g.m.Merge(g.prefix+path, handler)
}
