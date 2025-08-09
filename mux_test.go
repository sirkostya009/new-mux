package httx

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"slices"
)

type readWriter struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

var httpMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	http.MethodOptions,
	http.MethodTrace,
	MethodWild,
	"CUSTOM",
}

//go:embed LICENSE
var fsTestFilesystem embed.FS

func randomHTTPMethod() string {
	method := httpMethods[rand.Intn(len(httpMethods)-1)]

	for method == MethodWild {
		method = httpMethods[rand.Intn(len(httpMethods)-1)]
	}

	return method
}

var zeroTCPAddr = &net.TCPAddr{
	IP: net.IPv4zero,
}

func (rw *readWriter) Close() error {
	return nil
}

func (rw *readWriter) Read(b []byte) (int, error) {
	return rw.r.Read(b)
}

func (rw *readWriter) Write(b []byte) (int, error) {
	return rw.w.Write(b)
}

func (rw *readWriter) RemoteAddr() net.Addr {
	return zeroTCPAddr
}

func (rw *readWriter) LocalAddr() net.Addr {
	return zeroTCPAddr
}

func (rw *readWriter) SetReadDeadline(t time.Time) error {
	return nil
}

func (rw *readWriter) SetWriteDeadline(t time.Time) error {
	return nil
}

type assertFn func(rw *readWriter)

func assertWithTestServer(t *testing.T, uri string, handler http.Handler, fn assertFn) {
	s := httptest.NewServer(handler)
	defer s.Close()

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString(uri)
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout")
	}

	fn(rw)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

func TestRouter(t *testing.T) {
	router := NewMux()

	routed := false
	router.Handle(http.MethodGet, "/user/{name}", func(w http.ResponseWriter, r *http.Request) error {
		routed = true
		want := "gopher"

		param := r.PathValue("name")

		if param == "" {
			t.Fatalf("wrong wildcard values: param value is empty")
		}

		if param != want {
			t.Fatalf("wrong wildcard values: want %s, got %s", want, param)
		}

		return nil
	})

	router.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/user/gopher", nil),
	)

	if !routed {
		t.Fatal("routing failed")
	}
}

func TestRouterAPI(t *testing.T) {
	var handled, get, head, post, put, patch, delete, connect, options, trace, any bool

	httpHandler := func(w http.ResponseWriter, r *http.Request) error {
		handled = true
		return nil
	}

	router := NewMux()
	router.GET("/GET", func(w http.ResponseWriter, r *http.Request) error {
		get = true
		return nil
	})
	router.HEAD("/HEAD", func(w http.ResponseWriter, r *http.Request) error {
		head = true
		return nil
	})
	router.POST("/POST", func(w http.ResponseWriter, r *http.Request) error {
		post = true
		return nil
	})
	router.PUT("/PUT", func(w http.ResponseWriter, r *http.Request) error {
		put = true
		return nil
	})
	router.PATCH("/PATCH", func(w http.ResponseWriter, r *http.Request) error {
		patch = true
		return nil
	})
	router.DELETE("/DELETE", func(w http.ResponseWriter, r *http.Request) error {
		delete = true
		return nil
	})
	router.CONNECT("/CONNECT", func(w http.ResponseWriter, r *http.Request) error {
		connect = true
		return nil
	})
	router.OPTIONS("/OPTIONS", func(w http.ResponseWriter, r *http.Request) error {
		options = true
		return nil
	})
	router.TRACE("/TRACE", func(w http.ResponseWriter, r *http.Request) error {
		trace = true
		return nil
	})
	router.ANY("/ANY", func(w http.ResponseWriter, r *http.Request) error {
		any = true
		return nil
	})

	router.Handle(http.MethodGet, "/Handler", httpHandler)

	var request = func(method, path string) {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	}

	request(http.MethodGet, "/GET")
	if !get {
		t.Error("routing GET failed")
	}

	request(http.MethodHead, "/HEAD")
	if !head {
		t.Error("routing HEAD failed")
	}

	request(http.MethodPost, "/POST")
	if !post {
		t.Error("routing POST failed")
	}

	request(http.MethodPut, "/PUT")
	if !put {
		t.Error("routing PUT failed")
	}

	request(http.MethodPatch, "/PATCH")
	if !patch {
		t.Error("routing PATCH failed")
	}

	request(http.MethodDelete, "/DELETE")
	if !delete {
		t.Error("routing DELETE failed")
	}

	request(http.MethodConnect, "/CONNECT")
	if !connect {
		t.Error("routing CONNECT failed")
	}

	request(http.MethodOptions, "/OPTIONS")
	if !options {
		t.Error("routing OPTIONS failed")
	}

	request(http.MethodTrace, "/TRACE")
	if !trace {
		t.Error("routing TRACE failed")
	}

	request(http.MethodGet, "/Handler")
	if !handled {
		t.Error("routing Handler failed")
	}

	for _, method := range httpMethods {
		request(method, "/ANY")
		if !any {
			t.Errorf("routing ANY failed - Method: %s", method)
		}

		any = false
	}
}

func TestRouterInvalidInput(t *testing.T) {
	router := NewMux()

	handle := func(http.ResponseWriter, *http.Request) error { return nil }

	recv := catchPanic(func() {
		router.Handle("", "/", handle)
	})
	if recv == nil {
		t.Fatal("registering empty method did not panic")
	}

	recv = catchPanic(func() {
		router.GET("", handle)
	})
	if recv == nil {
		t.Fatal("registering empty path did not panic")
	}

	recv = catchPanic(func() {
		router.GET("noSlashRoot", handle)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}

	recv = catchPanic(func() {
		router.GET("/", nil)
	})
	if recv == nil {
		t.Fatal("registering nil handler did not panic")
	}
}

func TestRouterRegexUserValues(t *testing.T) {
	mux := NewMux()
	mux.GET("/metrics", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	v4 := NewMux()
	id := NewMux()
	id.GET("/click", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	v4.Merge("/{id:^[1-9]\\d*}", id)
	mux.Merge("/v4", v4)

	req := httptest.NewRequest(http.MethodGet, "/v4/123/click", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	v1 := req.PathValue("id")
	if v1 != "123" {
		t.Fatalf(`expected "123" in user value, got %q`, v1)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	if v1 != "123" {
		t.Fatalf(`expected "123" in user value after second call, got %q`, v1)
	}
}

func TestRouterChaining(t *testing.T) {
	router1 := NewMux()
	router2 := NewMux()
	router1.OnNotFound = router2.ServeHTTP

	fooHit := false
	router1.POST("/foo", func(w http.ResponseWriter, r *http.Request) error {
		fooHit = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	barHit := false
	router2.POST("/bar", func(w http.ResponseWriter, r *http.Request) error {
		barHit = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/foo", nil)
	rec := httptest.NewRecorder()
	router1.ServeHTTP(rec, req)

	if !(rec.Result().StatusCode == http.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	req = httptest.NewRequest(http.MethodPost, "/bar", nil)
	rec = httptest.NewRecorder()
	router1.ServeHTTP(rec, req)

	if !(rec.Result().StatusCode == http.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	req = httptest.NewRequest(http.MethodPost, "/qax", nil)
	rec = httptest.NewRecorder()
	router1.ServeHTTP(rec, req)

	if !(rec.Result().StatusCode == http.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

// func TestRouterMutable(t *testing.T) {
// 	handler1 := func(http.ResponseWriter, *http.Request) error { return nil }
// 	handler2 := func(http.ResponseWriter, *http.Request) error { return nil }

// 	router := NewMux()
// 	router.Mutable(true)

// 	if !router.treeMutable {
// 		t.Errorf("Router.treesMutables is false")
// 	}

// 	for _, method := range httpMethods {
// 		router.Handle(method, "/", handler1)
// 	}

// 	for method := range router.trees {
// 		if !router.trees[method].Mutable {
// 			t.Errorf("Method %d - Mutable == %v, want %v", method, router.trees[method].Mutable, true)
// 		}
// 	}

// 	routes := []string{
// 		"/",
// 		"/api/{version}",
// 		"/{filepath:*}",
// 		"/user{user:.*}",
// 	}

// 	router = NewMux()

// 	for _, route := range routes {
// 		for _, method := range httpMethods {
// 			router.Handle(method, route, handler1)
// 		}

// 		for _, method := range httpMethods {
// 			err := catchPanic(func() {
// 				router.Handle(method, route, handler2)
// 			})

// 			if err == nil {
// 				t.Errorf("Mutable 'false' - Method %s - Route %s - Expected panic", method, route)
// 			}

// 			h, _ := router.Lookup(method, route, nil)
// 			if reflect.ValueOf(h).Pointer() != reflect.ValueOf(handler1).Pointer() {
// 				t.Errorf("Mutable 'false' - Method %s - Route %s - Handler updated", method, route)
// 			}
// 		}

// 		router.Mutable(true)

// 		for _, method := range httpMethods {
// 			err := catchPanic(func() {
// 				router.Handle(method, route, handler2)
// 			})

// 			if err != nil {
// 				t.Errorf("Mutable 'true' - Method %s - Route %s - Unexpected panic: %v", method, route, err)
// 			}

// 			h, _ := router.Lookup(method, route, nil)
// 			if reflect.ValueOf(h).Pointer() != reflect.ValueOf(handler2).Pointer() {
// 				t.Errorf("Method %s - Route %s - Handler is not updated", method, route)
// 			}
// 		}

// 		router.Mutable(false)
// 	}
// }

func TestRouterOPTIONS(t *testing.T) {
	handlerFunc := func(http.ResponseWriter, *http.Request) error { return nil }

	router := NewMux()
	router.POST("/path", handlerFunc)

	var checkHandling = func(path, expectedAllowed string, expectedStatusCode int) {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if !(rec.Result().StatusCode == expectedStatusCode) {
			t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", rec.Result().StatusCode, rec.Result().Header)
		} else if allow := (rec.Result().Header.Values("Allow")); strings.Join(allow, ", ") != expectedAllowed {
			t.Error("unexpected Allow header value:", allow)
		}
	}

	// test not allowed
	// * (server)
	checkHandling("*", "OPTIONS, POST", http.StatusOK)

	// path
	checkHandling("/path", "OPTIONS, POST", http.StatusOK)

	req := httptest.NewRequest(http.MethodOptions, "/doesnotexist", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !(rec.Result().StatusCode == http.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", rec.Result().StatusCode, rec.Result().Header)
	}

	// add another method
	router.GET("/path", handlerFunc)

	// set a global OPTIONS handler
	router.GlobalOPTIONS = func(w http.ResponseWriter, r *http.Request) {
		// Adjust status code to 204
		w.WriteHeader(http.StatusNoContent)
	}

	// test again
	// * (server)
	checkHandling("*", "GET, OPTIONS, POST", http.StatusNoContent)

	// path
	checkHandling("/path", "GET, OPTIONS, POST", http.StatusNoContent)

	// custom handler
	var custom bool
	router.OPTIONS("/path", func(w http.ResponseWriter, r *http.Request) error {
		custom = true
		return nil
	})

	// test again
	// * (server)
	checkHandling("*", "GET, OPTIONS, POST", http.StatusNoContent)
	if custom {
		t.Error("custom handler called on *")
	}

	req = httptest.NewRequest(http.MethodOptions, "/path", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !(rec.Result().StatusCode == http.StatusNoContent) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", rec.Result().StatusCode, rec.Result().Header)
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(http.ResponseWriter, *http.Request) error { return nil }

	router := NewMux()
	router.POST("/path", handlerFunc)

	var checkHandling = func(path, expectedAllowed string, expectedStatusCode int) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if !(rec.Result().StatusCode == expectedStatusCode) {
			t.Errorf("NotAllowed handling failed:: Code=%d, Header=%v", rec.Result().StatusCode, rec.Result().Header)
		} else if allow := (rec.Result().Header.Values("Allow")); strings.Join(allow, ", ") != expectedAllowed {
			t.Error("unexpected Allow header value:", allow)
		}
	}

	// test not allowed
	checkHandling("/path", "OPTIONS, POST", http.StatusMethodNotAllowed)

	// add another method
	router.DELETE("/path", handlerFunc)
	router.OPTIONS("/path", handlerFunc) // must be ignored

	// test again
	checkHandling("/path", "DELETE, OPTIONS, POST", http.StatusMethodNotAllowed)

	// test custom handler
	responseText := "custom method"
	router.OnMethodNotAllowed = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(responseText))
	}

	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	b, _ := io.ReadAll(rec.Result().Body)
	if got := string(b); !(got == responseText) {
		t.Errorf("unexpected response got %q want %q", got, responseText)
	}
	if rec.Result().StatusCode != http.StatusTeapot {
		t.Errorf("unexpected response code %d want %d", rec.Result().StatusCode, http.StatusTeapot)
	}
	if allow := (rec.Result().Header.Get("Allow")); allow != "DELETE, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func testRouterNotFoundByMethod(t *testing.T, method string) {
	handlerFunc := func(http.ResponseWriter, *http.Request) error { return nil }

	router := NewMux()
	router.Handle(method, "/path", handlerFunc)
	router.Handle(method, "/dir/", handlerFunc)
	router.Handle(method, "/", handlerFunc)
	router.Handle(method, "/{proc}/StaTus", handlerFunc)
	router.Handle(method, "/USERS/{name}/enTRies/", handlerFunc)
	router.Handle(method, "/static/{filepath:*}", handlerFunc)

	reqMethod := method
	if method == MethodWild {
		reqMethod = randomHTTPMethod()
	}

	// Moved Permanently, request with GET method
	expectedCode := http.StatusMovedPermanently
	switch {
	case reqMethod == http.MethodConnect:
		// CONNECT method does not allow redirects, so Not Found (404)
		expectedCode = http.StatusNotFound
	case reqMethod != http.MethodGet:
		// Permanent Redirect, request with same method
		expectedCode = http.StatusPermanentRedirect
	}

	type testRoute struct {
		route    string
		code     int
		location string
	}

	testRoutes := []testRoute{
		// {"", http.StatusOK, ""},                                  // TSR +/ (Not clean by router, this path is cleaned by fasthttp `ctx.Path()`)
		{"/../path", expectedCode, ("/path")}, // CleanPath (Not clean by router, this path is cleaned by fasthttp `ctx.Path()`)
		{"/nope", http.StatusNotFound, ""},    // NotFound
	}

	if method != http.MethodConnect {
		testRoutes = append(testRoutes, []testRoute{
			{"/path/", expectedCode, "/path"},                                   // TSR -/
			{"/dir", expectedCode, "/dir/"},                                     // TSR +/
			{"/PATH", expectedCode, "/path"},                                    // Fixed Case
			{"/DIR/", expectedCode, "/dir/"},                                    // Fixed Case
			{"/PATH/", expectedCode, "/path"},                                   // Fixed Case -/
			{"/DIR", expectedCode, "/dir/"},                                     // Fixed Case +/
			{"/paTh/?name=foo", expectedCode, "/path?name=foo"},                 // Fixed Case With Query Params +/
			{"/paTh?name=foo", expectedCode, "/path?name=foo"},                  // Fixed Case With Query Params +/
			{"/sergio/status/", expectedCode, "/sergio/StaTus"},                 // Fixed Case With Params -/
			{"/users/atreugo/eNtriEs", expectedCode, "/USERS/atreugo/enTRies/"}, // Fixed Case With Params +/
			{"/STatiC/test.go", expectedCode, "/static/test.go"},                // Fixed Case Wildcard
		}...)
	}

	for _, tr := range testRoutes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(reqMethod, tr.route, nil)

		router.ServeHTTP(rec, req)

		statusCode := rec.Result().StatusCode
		location := (rec.Result().Header.Get("Location"))

		if !(statusCode == tr.code && (statusCode == http.StatusNotFound || location == tr.location)) {
			fn := t.Errorf
			msg := "NotFound handling route '%s' failed: Method=%s, ReqMethod=%s, Code=%d, ExpectedCode=%d, Header=%v"

			if runtime.GOOS == "windows" && strings.HasPrefix(tr.route, "/../") {
				// See: https://github.com/valyala/fasthttp/issues/1226
				// Not fail, because it is a known issue.
				fn = t.Logf
				msg = "ERROR: " + msg
			}

			fn(msg, tr.route, method, reqMethod, statusCode, tr.code, location)
		}
	}

	// Test custom not found handler
	var notFound bool
	router.OnNotFound = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		notFound = true
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(reqMethod, "/nope", nil)
	router.ServeHTTP(rec, req)

	if !(rec.Result().StatusCode == http.StatusNotFound && notFound == true) {
		t.Errorf(
			"Custom NotFound handling failed: Method=%s, ReqMethod=%s, Code=%d, Header=%v",
			method, reqMethod, rec.Result().StatusCode, rec.Result().Header,
		)
	}
}

func TestRouterNotFound(t *testing.T) {
	for _, method := range httpMethods {
		testRouterNotFoundByMethod(t, method)
	}

	router := NewMux()
	handlerFunc := func(http.ResponseWriter, *http.Request) error { return nil }
	host := "fast"

	// Test other method than GET (want 308 instead of 301)
	router.PATCH("/path", handlerFunc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/path/?key=val", nil)
	req.Host = host
	router.ServeHTTP(rec, req)
	if !(rec.Result().StatusCode == http.StatusPermanentRedirect && (rec.Result().Header.Get("Location")) == ("/path?key=val")) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", rec.Result().StatusCode, rec.Result().Header)
	}

	// Test special case where no node for the prefix "/" exists
	router = NewMux()
	router.GET("/a", handlerFunc)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/", nil)
	router.ServeHTTP(rec, req)
	if !(rec.Result().StatusCode == http.StatusNotFound) {
		t.Errorf("NotFound handling route / failed: Code=%d", rec.Result().StatusCode)
	}
}

func TestRouterNotFound_MethodWild(t *testing.T) {
	postFound, anyFound := false, false

	router := NewMux()
	router.ANY("/{path:*}", func(w http.ResponseWriter, r *http.Request) error {
		anyFound = true
		return nil
	})
	router.POST("/specific", func(w http.ResponseWriter, r *http.Request) error {
		postFound = true
		return nil
	})

	for range 100 {
		router.Handle(
			randomHTTPMethod(),
			fmt.Sprintf("/%d", rand.Int63()),
			func(w http.ResponseWriter, r *http.Request) error { return nil },
		)
	}

	var request = func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		router.ServeHTTP(rec, req)
		return rec
	}

	for _, method := range httpMethods {
		rec := request(method, "/specific")

		if method == http.MethodPost {
			if !postFound {
				t.Errorf("Method '%s': not found", method)
			}
		} else {
			if !anyFound {
				t.Errorf("Method 'ANY' not found with request method %s", method)
			}
		}

		status := rec.Result().StatusCode
		if status != http.StatusOK {
			t.Errorf("Response status code == %d, want %d", status, http.StatusOK)
		}

		postFound, anyFound = false, false
	}
}

func TestRouterPanicHandler(t *testing.T) {
	router := NewMux()
	panicHandled := false

	router.OnPanic = func(http.ResponseWriter, *http.Request, any) {
		panicHandled = true
	}

	router.Handle(http.MethodPut, "/user/{name}", func(w http.ResponseWriter, r *http.Request) error {
		panic("oops!")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/user/gopher", nil)

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	router.ServeHTTP(rec, req)

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

// func testRouterLookupByMethod(t *testing.T, method string) {
// 	reqMethod := method
// 	if method == MethodWild {
// 		reqMethod = randomHTTPMethod()
// 	}

// 	routed := false
// 	wantHandle := func(http.ResponseWriter, *http.Request) error {
// 		routed = true
// 		return nil
// 	}
// 	wantParams := map[string]string{"name": "gopher"}

// 	router := NewMux()
// 	req := httptest.NewRequest(http.MethodGet, "/", nil)

// 	// try empty router first
// 	handle, tsr := router.Lookup(reqMethod, "/nope", req)
// 	if handle != nil {
// 		t.Fatalf("Got handle for unregistered pattern: %v", handle)
// 	}
// 	if tsr {
// 		t.Error("Got wrong TSR recommendation!")
// 	}

// 	// insert route and try again
// 	router.Handle(method, "/user/{name}", wantHandle)
// 	handle, _ = router.Lookup(reqMethod, "/user/gopher", req)
// 	if handle == nil {
// 		t.Fatal("Got no handle!")
// 	} else {
// 		handle(nil)
// 		if !routed {
// 			t.Fatal("Routing failed!")
// 		}
// 	}

// 	for expectedKey, expectedVal := range wantParams {
// 		if req.PathValue(expectedKey) != expectedVal {
// 			t.Errorf("The values %s = %s is not save in context", expectedKey, expectedVal)
// 		}
// 	}

// 	routed = false

// 	// route without param
// 	router.Handle(method, "/user", wantHandle)
// 	handle, _ = router.Lookup(reqMethod, "/user", req)
// 	if handle == nil {
// 		t.Fatal("Got no handle!")
// 	} else {
// 		handle(nil)
// 		if !routed {
// 			t.Fatal("Routing failed!")
// 		}
// 	}

// 	for expectedKey, expectedVal := range wantParams {
// 		if req.PathValue(expectedKey) != expectedVal {
// 			t.Errorf("The values %s = %s is not save in context", expectedKey, expectedVal)
// 		}
// 	}

// 	handle, tsr = router.Lookup(reqMethod, "/user/gopher/", req)
// 	if handle != nil {
// 		t.Fatalf("Got handle for unregistered pattern: %v", handle)
// 	}
// 	if !tsr {
// 		t.Error("Got no TSR recommendation!")
// 	}

// 	handle, tsr = router.Lookup(reqMethod, "/nope", req)
// 	if handle != nil {
// 		t.Fatalf("Got handle for unregistered pattern: %v", handle)
// 	}
// 	if tsr {
// 		t.Error("Got wrong TSR recommendation!")
// 	}
// }

// func TestRouterLookup(t *testing.T) {
// 	for _, method := range httpMethods {
// 		testRouterLookupByMethod(t, method)
// 	}
// }

// func TestRouterMatchedRoutePath(t *testing.T) {
// 	route1 := "/user/{name}"
// 	routed1 := false
// 	handle1 := func(w http.ResponseWriter, r *http.Request) error {
// 		route := r.PathValue(MatchedRoutePathParam)
// 		if route != route1 {
// 			t.Fatalf("Wrong matched route: want %s, got %s", route1, route)
// 		}
// 		routed1 = true
// 	}

// 	route2 := "/user/{name}/details"
// 	routed2 := false
// 	handle2 := func(w http.ResponseWriter, r *http.Request) error {
// 		route := r.PathValue(MatchedRoutePathParam)
// 		if route != route2 {
// 			t.Fatalf("Wrong matched route: want %s, got %s", route2, route)
// 		}
// 		routed2 = true
// 		return nil
// 	}

// 	route3 := "/"
// 	routed3 := false
// 	handle3 := func(w http.ResponseWriter, r *http.Request) error {
// 		route := r.PathValue(MatchedRoutePathParam)
// 		if route != route3 {
// 			t.Fatalf("Wrong matched route: want %s, got %s", route3, route)
// 		}
// 		routed3 = true
// 		return nil
// 	}

// 	router := NewMux()
// 	router.SaveMatchedRoutePath = true
// 	router.Handle(http.MethodGet, route1, handle1)
// 	router.Handle(http.MethodGet, route2, handle2)
// 	router.Handle(http.MethodGet, route3, handle3)

// 	rec := httptest.NewRecorder()
// 	req := httptest.NewRequest(http.MethodGet, "/user/gopher", nil)
// 	router.ServeHTTP(rec, req)

// 	if !routed1 || routed2 || routed3 {
// 		t.Fatal("Routing failed!")
// 	}

// 	rec = httptest.NewRecorder()
// 	req = httptest.NewRequest(http.MethodGet, "/user/gopher/details", nil)
// 	router.ServeHTTP(rec, req)
// 	if !routed2 || routed3 {
// 		t.Fatal("Routing failed!")
// 	}

// 	rec = httptest.NewRecorder()
// 	req = httptest.NewRequest(http.MethodGet, "/", nil)
// 	router.ServeHTTP(rec, req)
// 	if !routed3 {
// 		t.Fatal("Routing failed!")
// 	}
// }

// func TestRouterServeFiles(t *testing.T) {
// 	r := NewMux()

// 	recv := catchPanic(func() {
// 		r.ServeFiles("/noFilepath", os.TempDir())
// 	})
// 	if recv == nil {
// 		t.Fatal("registering path not ending with '{filepath:*}' did not panic")
// 	}

// 	body := []byte("fake ico")
// 	if err := os.WriteFile(os.TempDir()+"/favicon.ico", body, 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	r.ServeFiles("/{filepath:*}", os.TempDir())

// 	assertWithTestServer(t, "GET /favicon.ico HTTP/1.1\r\n\r\n", r, func(rw *readWriter) {
// 		br := bufio.NewReader(&rw.w)
// 		var resp http.Response
// 		if err := resp.Read(br); err != nil {
// 			t.Fatalf("Unexpected error when reading response: %s", err)
// 		}
// 		if resp.Header.StatusCode() != 200 {
// 			t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 200)
// 		}
// 		if !bytes.Equal(resp.Body(), body) {
// 			t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
// 		}
// 	})
// }

// func TestRouterServeFS(t *testing.T) {
// 	r := NewMux()

// 	recv := catchPanic(func() {
// 		r.ServeFS("/noFilepath", fsTestFilesystem)
// 	})
// 	if recv == nil {
// 		t.Fatal("registering path not ending with '{filepath:*}' did not panic")
// 	}

// 	body, err := os.ReadFile("LICENSE")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	r.ServeFS("/{filepath:*}", fsTestFilesystem)

// 	assertWithTestServer(t, "GET /LICENSE HTTP/1.1\r\n\r\n", r.Handler, func(rw *readWriter) {
// 		br := bufio.NewReader(&rw.w)
// 		var resp http.Response
// 		if err := resp.Read(br); err != nil {
// 			t.Fatalf("Unexpected error when reading response: %s", err)
// 		}
// 		if resp.Header.StatusCode() != 200 {
// 			t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 200)
// 		}
// 		if !bytes.Equal(resp.Body(), body) {
// 			t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
// 		}
// 	})
// }

// func TestRouterServeFilesCustom(t *testing.T) {
// 	r := NewMux()

// 	root := os.TempDir()

// 	fs := &http.FS{
// 		Root: root,
// 	}

// 	recv := catchPanic(func() {
// 		r.ServeFilesCustom("/noFilepath", fs)
// 	})
// 	if recv == nil {
// 		t.Fatal("registering path not ending with '{filepath:*}' did not panic")
// 	}
// 	body := []byte("fake ico")
// 	ioutil.WriteFile(root+"/favicon.ico", body, 0644)

// 	r.ServeFilesCustom("/{filepath:*}", fs)

// 	assertWithTestServer(t, "GET /favicon.ico HTTP/1.1\r\n\r\n", r.Handler, func(rw *readWriter) {
// 		br := bufio.NewReader(&rw.w)
// 		var resp http.Response
// 		if err := resp.Read(br); err != nil {
// 			t.Fatalf("Unexpected error when reading response: %s", err)
// 		}
// 		if resp.Header.StatusCode() != 200 {
// 			t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 200)
// 		}
// 		if !bytes.Equal(resp.Body(), body) {
// 			t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
// 		}
// 	})
// }

func TestRouterList(t *testing.T) {
	expected := map[string][]string{
		"GET":    {"/bar"},
		"PATCH":  {"/foo"},
		"POST":   {"/v1/users/{name}/{surname?}"},
		"DELETE": {"/v1/users/{id?}"},
	}

	r := NewMux()
	r.GET("/bar", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	r.PATCH("/foo", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	v1 := NewMux()
	v1.POST("/users/{name}/{surname?}", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	v1.DELETE("/users/{id?}", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	r.Merge("/v1", v1)

	result := r.List()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Router.List() == %v, want %v", result, expected)
	}

}

func TestRouterSamePrefixParamRoute(t *testing.T) {
	var id1, id2, id3, pageSize, page, iid string
	var routed1, routed2, routed3 bool

	r := NewMux()
	v1 := NewMux()
	v1.GET("/foo/{id}/{pageSize}/{page}", func(w http.ResponseWriter, r *http.Request) error {
		id1 = r.PathValue("id")
		pageSize = r.PathValue("pageSize")
		page = r.PathValue("page")
		routed1 = true
		return nil
	})
	v1.GET("/foo/{id}/{iid}", func(w http.ResponseWriter, r *http.Request) error {
		id2 = r.PathValue("id")
		iid = r.PathValue("iid")
		routed2 = true
		return nil
	})
	v1.GET("/foo/{id}", func(w http.ResponseWriter, r *http.Request) error {
		id3 = r.PathValue("id")
		routed3 = true
		return nil
	})
	r.Merge("/v1", v1)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/foo/1/20/4", nil)
	r.ServeHTTP(rec, req)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/foo/2/3", nil)
	r.ServeHTTP(rec, req)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/foo/v3", nil)
	r.ServeHTTP(rec, req)

	if !routed1 {
		t.Error("/foo/{id}/{pageSize}/{page} not routed.")
	}
	if !routed2 {
		t.Error("/foo/{id}/{iid} not routed")
	}

	if !routed3 {
		t.Error("/foo/{id} not routed")
	}

	if id1 != "1" {
		t.Errorf("/foo/{id}/{pageSize}/{page} id expect: 1 got %s", id1)
	}

	if pageSize != "20" {
		t.Errorf("/foo/{id}/{pageSize}/{page} pageSize expect: 20 got %s", pageSize)
	}

	if page != "4" {
		t.Errorf("/foo/{id}/{pageSize}/{page} page expect: 4 got %s", page)
	}

	if id2 != "2" {
		t.Errorf("/foo/{id}/{iid} id expect: 2 got %s", id2)
	}

	if iid != "3" {
		t.Errorf("/foo/{id}/{iid} iid expect: 3 got %s", iid)
	}

	if id3 != "v3" {
		t.Errorf("/foo/{id} id expect: v3 got %s", id3)
	}
}

func TestGetOptionalPath(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	expected := []struct {
		path    string
		tsr     bool
		handler HandlerFunc
	}{
		{"/show/{name}", false, handler},
		{"/show/{name}/", true, nil},
		{"/show/{name}/{surname}", false, handler},
		{"/show/{name}/{surname}/", true, nil},
		{"/show/{name}/{surname}/at", false, handler},
		{"/show/{name}/{surname}/at/", true, nil},
		{"/show/{name}/{surname}/at/{address}", false, handler},
		{"/show/{name}/{surname}/at/{address}/", true, nil},
		{"/show/{name}/{surname}/at/{address}/{id}", false, handler},
		{"/show/{name}/{surname}/at/{address}/{id}/", true, nil},
		{"/show/{name}/{surname}/at/{address}/{id}/{phone:.*}", false, handler},
		{"/show/{name}/{surname}/at/{address}/{id}/{phone:.*}/", true, nil},
	}

	r := NewMux()
	r.GET("/show/{name}/{surname?}/at/{address?}/{id}/{phone?:.*}", handler)

	for _, e := range expected {
		req := &http.Request{}

		h, tsr := r.trees[r.methodIndexOf("GET")].Get(e.path, req)

		if tsr != e.tsr {
			t.Errorf("TSR (path: %s) == %v, want %v", e.path, tsr, e.tsr)
		}

		if h != nil && e.handler != nil && reflect.ValueOf(h) != reflect.ValueOf(e.handler) {
			t.Errorf("Handler (path: %s) == %p, want %p", e.path, h, e.handler)
		}
	}

	tests := []struct {
		path          string
		optionalPaths []string
	}{
		{"/hello", nil},
		{"/{name}", nil},
		{"/{name?:[a-zA-Z]{5}}", []string{"/", "/{name:[a-zA-Z]{5}}"}},
		{"/{filepath:^(?!api).*}", nil},
		{"/static/{filepath?:^(?!api).*}", []string{"/static", "/static/{filepath:^(?!api).*}"}},
		{"/show/{name?}", []string{"/show", "/show/{name}"}},
	}

	for _, test := range tests {
		optionalPaths := getOptionalPaths(test.path)

		if len(optionalPaths) != len(test.optionalPaths) {
			t.Errorf("getOptionalPaths() len == %d, want %d", len(optionalPaths), len(test.optionalPaths))
		}

		for _, wantPath := range test.optionalPaths {
			if !slices.Contains(optionalPaths, wantPath) {
				t.Errorf("The optional path is not returned for '%s': %s", test.path, wantPath)
			}
		}
	}
}
