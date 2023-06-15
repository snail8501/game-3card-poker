package src

import (
	"game-3-card-poker/server/config"
	"log"
	"net/http"
)

// RequestHandler represents an request handler.
type RequestHandler func(*config.ServerConfig, http.ResponseWriter, *http.Request)

// Middleware represents a fasthttp middleware.
type Middleware = func(next RequestHandler) RequestHandler

type HttpHandler func(http.ResponseWriter, *http.Request)

// Bridge represents the func signature that returns a fasthttp.RequestHandler given a RequestHandler allowing it to
// bridge between the two handlers.
type Bridge = func(RequestHandler) HttpHandler

// BridgeBuilder is used to build a Bridge.
type BridgeBuilder struct {
	config          *config.ServerConfig
	postMiddlewares []Middleware
}

// NewBridgeBuilder creates a new BridgeBuilder.
func NewBridgeBuilder(c *config.ServerConfig) *BridgeBuilder {
	return &BridgeBuilder{config: c}
}

// WithPostMiddlewares sets the Middleware's used with this BridgeBuilder which are applied after the actual
// Bridge.
func (b *BridgeBuilder) WithPostMiddlewares(middlewares ...Middleware) *BridgeBuilder {
	b.postMiddlewares = middlewares
	return b
}

// Build and return the Bridge configured by this BridgeBuilder.
func (b *BridgeBuilder) Build() Bridge {
	return func(next RequestHandler) HttpHandler {
		for i := len(b.postMiddlewares) - 1; i >= 0; i-- {
			next = b.postMiddlewares[i](next)
		}

		bridge := func(w http.ResponseWriter, r *http.Request) {
			log.Print("Request: -->> ", r)
			next(b.config, w, r)
		}
		return bridge
	}
}
