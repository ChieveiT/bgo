package bgo

import (
	"context"
	"fmt"
	"net/http"
	"net"

	httprouter "github.com/julienschmidt/httprouter"
	sentry "github.com/onrik/logrus/sentry"
	cors "github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

// Router thin wrapper for httprouter.Router
type Router struct {
	prefix      string
	middlewares []Middleware
	cors        *cors.Cors
	*httprouter.Router
}

// New create a bgo Router
func New() *Router {
	// sentry
	if sentryDSN, ok := Config["sentry"].(string); ok {
		sentryHook := sentry.NewHook(sentryDSN, log.ErrorLevel, log.PanicLevel, log.FatalLevel)
		Log.AddHook(sentryHook)
	}

	return &Router{
		Router: httprouter.New(),
		middlewares: []Middleware{
			logMiddleware,
			recoverMiddleware,
		},
	}
}

// ListenAndServe is a shortcut for http.ListenAndServe
func (r *Router) ListenAndServe() {
	port := "8080"
	if Config["port"] != nil {
		switch p := Config["port"].(type) {
		case int:
			port = fmt.Sprintf("%d", p)
		case string:
			port = p
		default:
			Log.Fatal("port must be int or string")
		}
	}

	Log.WithField("port", port).Info("http.ListenAndServe")
	Log.Fatal(http.ListenAndServe(":"+port, r))
}

// Serve is a shortcut for http.Serve
func (r *Router) Serve(l net.Listener) {
	Log.Fatal(http.Serve(l, r))
}

// Prefix append prefix
func (r *Router) Prefix(p string) *Router {
	new := *r
	new.prefix = new.prefix + p
	return &new
}

// Middlewares register middlewares
func (r *Router) Middlewares(layers ...Middleware) *Router {
	new := *r
	new.middlewares = append(r.middlewares, layers...)
	return &new
}

// CORS register middlewares
func (r *Router) CORS(c *cors.Cors) *Router {
	new := *r
	new.cors = c
	return &new
}

// Handle func
type Handle = func(context.Context)

// Middleware func
type Middleware = func(context.Context, Handle)

// HTTP context
type HTTP struct {
	Response http.ResponseWriter
	Request  *http.Request
	Params   httprouter.Params
}

// Handle define a route
func (r *Router) Handle(method, path string, middlewaresAndHandle ...interface{}) *Router {
	l := len(middlewaresAndHandle)
	if l == 0 {
		Log.Panic("expect bgo.Handle")
	}

	var handle Handle
	switch h := middlewaresAndHandle[l-1].(type) {
	case Handle:
		handle = h
	case http.Handler:
		handle = func (ctx context.Context) {
			c := ctx.Value(CtxKey("http")).(*HTTP)
			h.ServeHTTP(c.Response, c.Request)
		}
	default:
		Log.Panicf("expect bgo.Handle or http.Handler, but get %T", middlewaresAndHandle[l-1])
	}

	middlewares := r.middlewares
	for i := 0; i < l-1; i++ {
		middleware, ok := middlewaresAndHandle[i].(Middleware)
		if !ok {
			Log.Panicf("expect bgo.Middleware, but get %T", middlewaresAndHandle[i])
		}
		middlewares = append(middlewares, middleware)
	}

	// handle wrapped with middlewares
	for i := len(middlewares) - 1; i >= 0; i-- {
		middleware := middlewares[i]
		next := handle
		handle = func(ctx context.Context) {
			middleware(ctx, next)
		}
	}

	// wrap it as httprouter.Handle
	// attach response, request, params to context
	hrHandle := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxKey("http"), &HTTP{w, r, ps})
		handle(ctx)
	}

	hr := r.Router

	// compatible with rs/cors
	if r.cors != nil {
		if method != "OPTIONS" {
			hr.Handle("OPTIONS", r.prefix+path, func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
				r.cors.HandlerFunc(w, req)
			})
		}

		innerHandle := hrHandle
		hrHandle = func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			r.cors.HandlerFunc(w, req)
			innerHandle(w, req, ps)
		}
	}

	hr.Handle(method, r.prefix+path, hrHandle)

	return r
}

// GET is a shortcut for router.Handle("GET", path, middlewaresAndHandle...)
func (r *Router) GET(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("GET", path, middlewaresAndHandle...)
}

// HEAD is a shortcut for router.Handle("HEAD", path, middlewaresAndHandle...)
func (r *Router) HEAD(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("HEAD", path, middlewaresAndHandle...)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, middlewaresAndHandle...)
func (r *Router) OPTIONS(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("OPTIONS", path, middlewaresAndHandle...)
}

// POST is a shortcut for router.Handle("POST", path, middlewaresAndHandle...)
func (r *Router) POST(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("POST", path, middlewaresAndHandle...)
}

// PUT is a shortcut for router.Handle("PUT", path, middlewaresAndHandle...)
func (r *Router) PUT(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("PUT", path, middlewaresAndHandle...)
}

// PATCH is a shortcut for router.Handle("PATCH", path, middlewaresAndHandle...)
func (r *Router) PATCH(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("PATCH", path, middlewaresAndHandle...)
}

// DELETE is a shortcut for router.Handle("DELETE", path, middlewaresAndHandle...)
func (r *Router) DELETE(path string, middlewaresAndHandle ...interface{}) *Router {
	return r.Handle("DELETE", path, middlewaresAndHandle...)
}
