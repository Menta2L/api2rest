package api2rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/Menta2L/api2rest/routing"
	"github.com/jinzhu/gorm"
)

const (
	defaultContentTypHeader = "application/vnd.api+json"
)

var (
	defaultLimit int64 = 25
)

type notAllowedHandler struct {
	API *API
}

func (n notAllowedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := errors.New("Method Not Allowed") //NewHTTPError(nil, "Method Not Allowed", http.StatusMethodNotAllowed)
	w.WriteHeader(http.StatusMethodNotAllowed)
	n.API.handleError(err, w, r)
}

// HandlerFunc for api2go middlewares
type HandlerFunc func(APIContexter, http.ResponseWriter, *http.Request)

// holding information
type information struct {
	prefix   string
	resolver URLResolver
}

func (i information) GetBaseURL() string {
	return i.resolver.GetBaseURL()
}

func (i information) GetPrefix() string {
	return i.prefix
}

// api resource
type resource struct {
	resourceType reflect.Type
	name         string
	api          *API
}

// API is a REST JSONAPI.
type API struct {
	DB               *gorm.DB
	ContentType      string
	router           routing.Routeable
	info             information
	resources        []resource
	middlewares      []HandlerFunc
	contextPool      sync.Pool
	contextAllocator APIContextAllocatorFunc
}

// middlewareChain executes the middleeware chain setup
func (api *API) middlewareChain(c APIContexter, w http.ResponseWriter, r *http.Request) {
	for _, middleware := range api.middlewares {
		middleware(c, w, r)
	}
}

func (api *API) addResource(prototype MarshalIdentifier) *resource {
	log.Printf("add res %v", prototype)
	resourceType := reflect.TypeOf(prototype)
	if resourceType.Kind() != reflect.Struct && resourceType.Kind() != reflect.Ptr {
		panic("pass an empty resource struct or a struct pointer to AddResource!")
	}

	var name string

	if resourceType.Kind() == reflect.Struct {
		name = resourceType.Name()
	} else {
		name = resourceType.Elem().Name()
	}

	// check if EntityNamer interface is implemented and use that as name
	entityName, ok := prototype.(EntityNamer)
	if ok {
		name = entityName.GetName()
	} else {
		name = Jsonify(Pluralize(name))
	}

	res := resource{
		resourceType: resourceType,
		name:         name,
		api:          api,
	}

	requestInfo := func(r *http.Request, api *API) *information {
		var info *information
		if resolver, ok := api.info.resolver.(RequestAwareURLResolver); ok {
			resolver.SetRequest(*r)
			info = &information{prefix: api.info.prefix, resolver: resolver}
		} else {
			info = &api.info
		}

		return info
	}

	prefix := strings.Trim(api.info.prefix, "/")
	baseURL := "/" + name
	if prefix != "" {
		baseURL = "/" + prefix + baseURL
	}
	api.router.Handle("OPTIONS", baseURL, func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		w.Header().Set("Allow", "GET,POST,PATCH,OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		api.contextPool.Put(c)
	})

	api.router.Handle("OPTIONS", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		w.Header().Set("Allow", "GET,PATCH,DELETE,OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		api.contextPool.Put(c)
	})

	api.router.Handle("GET", baseURL, func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		info := requestInfo(r, api)
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)

		err := res.handleIndex(c, w, r, *info)
		api.contextPool.Put(c)
		if err != nil {
			api.handleError(err, w, r)
		}
	})
	api.router.Handle("GET", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string) {
		info := requestInfo(r, api)
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		err := res.handleRead(c, w, r, params, *info)
		api.contextPool.Put(c)
		if err != nil {
			api.handleError(err, w, r)
		}
	})

	api.router.Handle("POST", baseURL, func(w http.ResponseWriter, r *http.Request, params map[string]string) {
		info := requestInfo(r, api)
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		err := res.handleCreate(c, w, r, info.prefix, *info)
		api.contextPool.Put(c)
		if err != nil {
			api.handleError(err, w, r)
		}
	})
	api.router.Handle("DELETE", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string) {
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		err := res.handleDelete(c, w, r, params)
		api.contextPool.Put(c)
		if err != nil {
			api.handleError(err, w, r)
		}
	})
	api.router.Handle("PATCH", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string) {
		info := requestInfo(r, api)
		c := api.contextPool.Get().(APIContexter)
		c.Reset()
		api.middlewareChain(c, w, r)
		err := res.handleUpdate(c, w, r, params, *info)
		api.contextPool.Put(c)
		if err != nil {
			api.handleError(err, w, r)
		}
	})
	return &res
}
func (api *API) handleError(err error, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `{"error":%q}`, err)
}
func (res *resource) handleRead(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, info information) error {

	id, err := strconv.ParseInt(params["id"], 10, 64)
	if err != nil {
		return err
	}
	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()

	if err := res.api.DB.First(newObj, id).Error; err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newObj)
	return nil
}

func (res *resource) handleIndex(c APIContexter, w http.ResponseWriter, r *http.Request, info information) error {
	var page int64
	var offset int64
	var limit int64
	limit = defaultLimit
	var err error
	queryParams := r.URL.Query()
	queryParams.Get("page")

	if queryParams.Get("page") != "" {
		page, err = strconv.ParseInt(queryParams.Get("page"), 10, 64)
		if err != nil {
			page = 1
		}
		if page < 1 {
			page = 1
		}
	}
	if queryParams.Get("limit") != "" {
		limit, err = strconv.ParseInt(queryParams.Get("limit"), 10, 64)
		if err != nil {
			limit = defaultLimit
		}
	}
	offset = (page - 1) * limit

	slice := reflect.New(reflect.SliceOf(res.resourceType)).Interface()
	if offset > 0 {
		err = res.api.DB.Limit(limit).Offset(offset).Find(slice).Error
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			return errors.New("Failed to get objects")
		}
	}
	err = res.api.DB.Limit(limit).Find(slice).Error
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		return errors.New("Failed to get objects")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slice)
	return nil
}
func (res *resource) handleCreate(c APIContexter, w http.ResponseWriter, r *http.Request, prefix string, info information) error {

	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newObj)
	if err != nil {
		return errors.New("Failed to decode json")
	}
	if err := res.api.DB.Create(newObj).Error; err != nil {
		return errors.New("Failed to create object")

	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newObj)
	return nil
}

func (res *resource) handleDelete(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string) error {
	id := params["id"]
	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()
	if err := res.api.DB.Where("id = ?", id).Delete(newObj).Error; err != nil {
		return errors.New("Failed to delete object")
	}
	return nil
}
func (res *resource) handleUpdate(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, info information) error {
	id, err := strconv.ParseInt(params["id"], 10, 64)
	if err != nil {
		return err
	}
	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()
	oldObj := reflect.New(resourceType).Interface()
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&newObj)
	if err != nil {
		log.Print(err)
		return errors.New("Failed to decode json")
	}
	if err := res.api.DB.First(oldObj, id).Error; err != nil {
		return errors.New("Unable to find object for update")
	}
	if err := res.api.DB.Model(oldObj).Updates(newObj).Error; err != nil {
		return errors.New("Failed to update object")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newObj)
	return nil
}

// allocateContext creates a context for the api.contextPool, saving allocations
func (api *API) allocateDefaultContext() APIContexter {
	return &APIContext{}
}
