package httx

import (
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unsafe"

	"github.com/sirkostya009/httx/radix"
)

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), 500)
}

func DefaultOnMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(405)
}

func DefaultOnNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
}

func DefaultOnPanic(w http.ResponseWriter, r *http.Request, a any) {
	slog.Error("panic recovered", slog.Any("message", a))
	w.WriteHeader(500)
}

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func (hf HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := hf(w, r); err != nil {
		DefaultErrorHandler(w, r, err)
	}
}

type Mux struct {
	// Centralized error handling for the Mux. Cannot be nil.
	OnError func(http.ResponseWriter, *http.Request, error)

	// Configurable http.HandlerFunc which is called when a request
	// cannot be routed.
	// If nil, OnNotFound is called instead.
	// The "Allow" header with allowed request methods is set before this handler
	// is called.
	OnMethodNotAllowed func(http.ResponseWriter, *http.Request)

	// Configurable http.Handler which is called when no matching route is
	// found. Cannot be nil.
	OnNotFound func(http.ResponseWriter, *http.Request)

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	OnPanic func(http.ResponseWriter, *http.Request, any)

	// An optional http.HandlerFunc that is called on automatic OPTIONS requests.
	// The handler is only called if its not nil and no OPTIONS
	// handler for the specific path was set.
	// The "Allowed" header is set before calling the handler.
	GlobalOPTIONS func(http.ResponseWriter, *http.Request)

	mw                 []func(HandlerFunc) HandlerFunc
	trees              []*radix.Tree
	customMethodsIndex map[string]int
	registeredPaths    map[string][]string
	globalAllowed      []string
	treeMutable        bool

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 308 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 308 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool
}

func NewMux() *Mux {
	return &Mux{
		trees:                 make([]*radix.Tree, 10),
		customMethodsIndex:    map[string]int{},
		registeredPaths:       map[string][]string{},
		RedirectTrailingSlash: true,
		RedirectFixedPath:     true,
		OnError:               DefaultErrorHandler,
		OnMethodNotAllowed:    DefaultOnMethodNotAllowed,
		OnNotFound:            DefaultOnNotFound,
		OnPanic:               DefaultOnPanic,
	}
}

func (m *Mux) Pre(mw ...func(HandlerFunc) HandlerFunc) {
	// clipping ensures we don't modify the original mw array in Merge
	m.mw = slices.Clip(append(m.mw, mw...))
}

// List returns all registered routes grouped by method
func (m *Mux) List() map[string][]string {
	return m.registeredPaths
}

// GET is a shortcut for router.Handle(http.MethodGet, path, handler)
func (m *Mux) GET(path string, handler HandlerFunc) {
	m.Handle(http.MethodGet, path, handler)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handler)
func (m *Mux) HEAD(path string, handler HandlerFunc) {
	m.Handle(http.MethodHead, path, handler)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handler)
func (m *Mux) POST(path string, handler HandlerFunc) {
	m.Handle(http.MethodPost, path, handler)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handler)
func (m *Mux) PUT(path string, handler HandlerFunc) {
	m.Handle(http.MethodPut, path, handler)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handler)
func (m *Mux) PATCH(path string, handler HandlerFunc) {
	m.Handle(http.MethodPatch, path, handler)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handler)
func (m *Mux) DELETE(path string, handler HandlerFunc) {
	m.Handle(http.MethodDelete, path, handler)
}

// CONNECT is a shortcut for router.Handle(http.MethodConnect, path, handler)
func (m *Mux) CONNECT(path string, handler HandlerFunc) {
	m.Handle(http.MethodConnect, path, handler)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handler)
func (m *Mux) OPTIONS(path string, handler HandlerFunc) {
	m.Handle(http.MethodOptions, path, handler)
}

// TRACE is a shortcut for router.Handle(http.MethodTrace, path, handler)
func (m *Mux) TRACE(path string, handler HandlerFunc) {
	m.Handle(http.MethodTrace, path, handler)
}

// ANY is a shortcut for router.Handle(router.MethodWild, path, handler)
//
// Requests with any method will route to this, unless a route with a distinct method was found.
func (m *Mux) ANY(path string, handler HandlerFunc) {
	m.Handle(MethodWild, path, handler)
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.OnPanic != nil {
		defer func() {
			if recv := recover(); recv != nil {
				m.OnPanic(w, r, recv)
			}
		}()
	}

	path := r.URL.Path

	if methodIndex := m.methodIndexOf(r.Method); methodIndex > -1 {
		if tree := m.trees[methodIndex]; tree != nil {
			if handler, tsr := tree.Get(path, r); handler != nil {
				handler.ServeHTTP(w, r)
				return
			} else if r.Method != http.MethodConnect && path != "/" {
				if ok := m.tryRedirect(w, r, tree, tsr, r.Method, path); ok {
					return
				}
			}
		}
	}

	// Try to search in the wild method tree
	if tree := m.trees[m.methodIndexOf(MethodWild)]; tree != nil {
		if handler, tsr := tree.Get(path, r); handler != nil {
			handler.ServeHTTP(w, r)
			return
		} else if r.Method != http.MethodConnect && path != "/" {
			if ok := m.tryRedirect(w, r, tree, tsr, r.Method, path); ok {
				return
			}
		}
	}

	if r.Method == http.MethodOptions && m.GlobalOPTIONS != nil {
		if allow := m.allowed(path, http.MethodOptions); len(allow) > 0 {
			w.Header()["Allow"] = allow
			m.GlobalOPTIONS(w, r)
			return
		}
	} else if m.OnMethodNotAllowed != nil {
		if allow := m.allowed(path, r.Method); len(allow) > 0 {
			w.Header()["Allow"] = allow
			m.OnMethodNotAllowed(w, r)
			return
		}
	}

	m.OnNotFound(w, r)
}

func (m *Mux) tryRedirect(w http.ResponseWriter, r *http.Request, tree *radix.Tree, tsr bool, method, path string) bool {
	// Moved Permanently, request with GET method
	code := http.StatusMovedPermanently
	if method != http.MethodGet {
		// Permanent Redirect, request with same method
		code = http.StatusPermanentRedirect
	}

	if tsr && m.RedirectTrailingSlash {
		uri := make([]byte, 0, len(r.RequestURI)+1)

		if len(path) > 1 && path[len(path)-1] == '/' {
			uri = append(uri, path[:len(path)-1]...)
		} else {
			uri = append(uri, path...)
			uri = append(uri, '/')
		}

		if len(r.URL.RawQuery) > 0 {
			uri = append(uri, '?')
			uri = append(uri, r.URL.RawQuery...)
		}

		w.WriteHeader(code)
		w.Header()["Location"] = []string{unsafe.String(&uri[0], len(uri))}

		return true
	}

	// Try to fix the request path
	if m.RedirectFixedPath {
		uri := make([]byte, 0, len(r.RequestURI)+1)
		found := tree.FindCaseInsensitivePath(
			strings.TrimSuffix(r.URL.Path, "."),
			m.RedirectTrailingSlash,
			&uri,
		)

		if found {
			if len(r.URL.RawQuery) > 0 {
				uri = append(uri, '?')
				uri = append(uri, r.URL.RawQuery...)
			}

			w.WriteHeader(code)
			w.Header()["Location"] = []string{unsafe.String(&uri[0], len(uri))}

			return true
		}
	}

	return false
}

func (m *Mux) Merge(prefix string, handler http.Handler) {
	switch h := handler.(type) {
	case *Mux:
		m2 := &Mux{}
		*m2 = *m
		m2.mw = append(m2.mw, h.mw...)
		m2.OnError = h.OnError
		for method, paths := range h.registeredPaths {
			for _, path := range paths {
				methodIndex := h.methodIndexOf(method)
				if h, _ := h.trees[methodIndex].Get(path, &http.Request{}); h != nil {
					fullPath := prefix + path
					if prefix != "" && path == "/" {
						fullPath = prefix
					}
					switch h := h.(type) {
					case HandlerFunc:
						m2.Handle(method, fullPath, h)
					default:
						m2.Merge(fullPath, h)
					}
				}
			}
		}
	default:
		if !strings.HasSuffix(prefix, "*") {
			panic("non-Mux merges must end with *")
		}
		noStar := prefix[:len(prefix)-1]
		m.Handle(MethodWild, prefix, func(w http.ResponseWriter, r *http.Request) error {
			// the exact copy of code from http.StripPrefix
			p := strings.TrimPrefix(r.URL.Path, noStar)
			rp := strings.TrimPrefix(r.URL.RawPath, noStar)
			if len(p) < len(r.URL.Path) && (r.URL.RawPath == "" || len(rp) < len(r.URL.RawPath)) {
				r2 := &http.Request{}
				*r2 = *r
				r2.URL = &url.URL{}
				*r2.URL = *r.URL
				r2.URL.Path = p
				r2.URL.RawPath = rp
				h.ServeHTTP(w, r2)
			} else {
				m.OnNotFound(w, r)
			}
			return nil
		})
	}
}

func (m *Mux) Handle(method, path string, handler HandlerFunc) {
	switch {
	case len(method) == 0:
		panic("method must not be empty")
	case handler == nil:
		panic("handler must not be nil")
	default:
		validatePath(path)
	}

	m.registeredPaths[method] = append(m.registeredPaths[method], path)

	methodIndex := m.methodIndexOf(method)
	if methodIndex == -1 {
		tree := radix.New()
		tree.Mutable = m.treeMutable

		m.trees = append(m.trees, tree)
		methodIndex = len(m.trees) - 1
		m.customMethodsIndex[method] = methodIndex
	}

	tree := m.trees[methodIndex]
	if tree == nil {
		tree = radix.New()
		tree.Mutable = m.treeMutable

		m.trees[methodIndex] = tree
		m.globalAllowed = m.allowed("*", "")
	}

	for _, mw := range m.mw {
		handler = mw(handler)
	}

	onerr := m.OnError
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err != nil {
			onerr(w, r, err)
		}
	})

	optionalPaths := getOptionalPaths(path)

	// if no optional paths, adds the original
	if len(optionalPaths) == 0 {
		tree.Add(path, stdHandler)
	} else {
		for _, p := range optionalPaths {
			tree.Add(p, stdHandler)
		}
	}
}

func (m *Mux) allowed(path, reqMethod string) (allow []string) {
	allowed := make([]string, 0, 9)

	if path == "*" || path == "/*" { // server-wide
		// empty method is used for internal calls to refresh the cache
		if reqMethod == "" {
			for method := range m.registeredPaths {
				if method == http.MethodOptions {
					continue
				}
				// Add request method to list of allowed methods
				allowed = append(allowed, method)
			}
		} else {
			return m.globalAllowed
		}
	} else { // specific path
		for method := range m.registeredPaths {
			// Skip the requested method - we already tried this one
			if method == reqMethod || method == http.MethodOptions {
				continue
			}

			handle, _ := m.trees[m.methodIndexOf(method)].Get(path, nil)
			if handle != nil {
				// Add request method to list of allowed methods
				allowed = append(allowed, method)
			}
		}
	}

	if len(allowed) > 0 {
		// Add request method to list of allowed methods
		allowed = append(allowed, http.MethodOptions)

		// Sort allowed methods.
		// sort.Strings(allowed) unfortunately causes unnecessary allocations
		// due to allowed being moved to the heap and interface conversion
		for i, l := 1, len(allowed); i < l; i++ {
			for j := i; j > 0 && allowed[j] < allowed[j-1]; j-- {
				allowed[j], allowed[j-1] = allowed[j-1], allowed[j]
			}
		}

		return allowed
	}

	return
}

// getOptionalPaths returns all possible paths when the original path
// has optional arguments
func getOptionalPaths(path string) []string {
	paths := make([]string, 0)

	start := 0
walk:
	for {
		if start >= len(path) {
			return paths
		}

		c := path[start]
		start++

		if c != '{' {
			continue
		}

		newPath := ""
		hasRegex := false
		questionMarkIndex := -1

		brackets := 0

		for end, c := range []byte(path[start:]) {
			switch c {
			case '{':
				brackets++
			case '}':
				if brackets > 0 {
					brackets--
					continue
				} else if questionMarkIndex == -1 {
					continue walk
				}

				end++
				newPath += path[questionMarkIndex+1 : start+end]

				path = path[:questionMarkIndex] + path[questionMarkIndex+1:] // remove '?'
				paths = append(paths, newPath)
				start += end - 1

				continue walk
			case ':':
				hasRegex = true
			case '?':
				if hasRegex {
					continue
				}

				questionMarkIndex = start + end
				newPath += path[:questionMarkIndex]

				if len(path[:start-2]) == 0 {
					// include the root slash because the param is in the first segment
					paths = append(paths, "/")
				} else if !slices.Contains(paths, path[:start-2]) {
					// include the path without the wildcard
					// -2 due to remove the '/' and '{'
					paths = append(paths, path[:start-2])
				}
			}
		}
	}
}

func validatePath(path string) {
	switch {
	case len(path) == 0 || !strings.HasPrefix(path, "/"):
		panic("path must begin with '/' in path '" + path + "'")
	}
}

// MethodWild wild HTTP method
const MethodWild = "*"

func (m *Mux) methodIndexOf(method string) int {
	switch method {
	case http.MethodGet:
		return 0
	case http.MethodHead:
		return 1
	case http.MethodPost:
		return 2
	case http.MethodPut:
		return 3
	case http.MethodPatch:
		return 4
	case http.MethodDelete:
		return 5
	case http.MethodConnect:
		return 6
	case http.MethodOptions:
		return 7
	case http.MethodTrace:
		return 8
	case MethodWild:
		return 9
	}

	if i, ok := m.customMethodsIndex[method]; ok {
		return i
	}

	return -1
}
