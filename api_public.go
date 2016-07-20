package api2rest

import (
	"net/http"

	"github.com/Menta2L/api2rest/routing"
	"github.com/jinzhu/gorm"
)

// Handler returns the http.Handler instance for the API.
func (api API) Handler() http.Handler {
	return api.router.Handler()
}

//Router returns the specified router on an api instance
func (api API) Router() routing.Routeable {
	return api.router
}
func (api *API) SetDb(db *gorm.DB) {
	api.DB = db
}

// SetContextAllocator custom implementation for making contexts
func (api *API) SetContextAllocator(allocator APIContextAllocatorFunc) {
	api.contextAllocator = allocator
}

// AddResource registers a data source for the given resource
// At least the CRUD interface must be implemented, all the other interfaces are optional.
// `resource` should be either an empty struct instance such as `Post{}` or a pointer to
// a struct such as `&Post{}`. The same type will be used for constructing new elements.
func (api *API) AddResource(prototype MarshalIdentifier) {
	api.addResource(prototype)
}

// UseMiddleware registers middlewares that implement the api2go.HandlerFunc
// Middleware is run before any generated routes.
func (api *API) UseMiddleware(middleware ...HandlerFunc) {
	api.middlewares = append(api.middlewares, middleware...)
}

// NewAPIWithResolver can be used to create an API with a custom URL resolver.
func NewAPIWithResolver(prefix string, resolver URLResolver) *API {
	handler := notAllowedHandler{}
	r := routing.NewHTTPRouter(prefix, &handler)
	api := newAPI(prefix, resolver, r)
	handler.API = api
	return api
}

// NewAPIWithBaseURL does the same as NewAPI with the addition of
// a baseURL which get's added in front of all generated URLs.
// For example http://localhost/v1/myResource/abc instead of /v1/myResource/abc
func NewAPIWithBaseURL(prefix string, baseURL string) *API {
	handler := notAllowedHandler{}
	staticResolver := NewStaticResolver(baseURL)
	r := routing.NewHTTPRouter(prefix, &handler)
	api := newAPI(prefix, staticResolver, r)
	handler.API = api
	return api
}

// NewAPI returns an initialized API instance
// `prefix` is added in front of all endpoints.
func NewAPI(prefix string) *API {
	handler := notAllowedHandler{}
	staticResolver := NewStaticResolver("")
	r := routing.NewHTTPRouter(prefix, &handler)
	api := newAPI(prefix, staticResolver, r)
	handler.API = api
	return api
}

// NewAPIWithRouting allows you to use a custom URLResolver, marshalers and custom routing
// if you want to use the default routing, you should use another constructor.
//
// If you don't need any of the parameters you can skip them with the defaults:
// the default for `prefix` would be `""`, which means there is no namespace for your api.
// although we suggest using one.
//
// if your api only answers to one url you can use a NewStaticResolver() as  `resolver`
func NewAPIWithRouting(prefix string, resolver URLResolver, router routing.Routeable) *API {
	return newAPI(prefix, resolver, router)
}

// newAPI is now an internal method that can be changed if params are changing
func newAPI(prefix string, resolver URLResolver, router routing.Routeable) *API {
	info := information{prefix: prefix, resolver: resolver}

	api := &API{
		ContentType:      defaultContentTypHeader,
		router:           router,
		info:             info,
		middlewares:      make([]HandlerFunc, 0),
		contextAllocator: nil,
	}

	api.contextPool.New = func() interface{} {
		if api.contextAllocator != nil {
			return api.contextAllocator(api)
		}
		return api.allocateDefaultContext()
	}

	return api
}
