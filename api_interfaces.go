package api2rest

import "net/http"

// MarshalIdentifier interface is necessary to give an element
// a unique ID. This interface must be implemented for
// marshal and unmarshal in order to let them store
// elements
type MarshalIdentifier interface {
	GetID() string
}

//URLResolver allows you to implement a static
//way to return a baseURL for all incoming
//requests for one api2go instance.
type URLResolver interface {
	GetBaseURL() string
}

// The EntityNamer interface can be opionally implemented to rename a struct. The name returned by
// GetName will be used for the route generation as well as the "type" field in all responses
type EntityNamer interface {
	GetName() string
}

// RequestAwareURLResolver allows you to dynamically change
// generated urls.
//
// This is particulary useful if you have the same
// API answering to multiple domains, or subdomains
// e.g customer[1,2,3,4].yourapi.example.com
//
// SetRequest will always be called prior to
// the GetBaseURL() from `URLResolver` so you
// have to change the result value based on the last
// request.
type RequestAwareURLResolver interface {
	URLResolver
	SetRequest(http.Request)
}
