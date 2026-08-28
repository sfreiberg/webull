// Package query builds URL query strings for service packages. It omits unset
// values, so that an optional field left at its zero value is not sent as an
// empty parameter.
package query

import (
	"net/url"
	"strconv"
	"strings"
)

// Params accumulates query parameters. The zero value is ready to use.
type Params url.Values

// New returns an empty Params.
func New() Params { return Params{} }

// Set adds a string parameter unless it is empty.
func (p Params) Set(key, value string) {
	if value != "" {
		url.Values(p).Set(key, value)
	}
}

// SetList joins values with commas, which is how Webull accepts multi-value
// parameters such as symbol lists, unless the list is empty.
func (p Params) SetList(key string, values []string) {
	if len(values) > 0 {
		url.Values(p).Set(key, strings.Join(values, ","))
	}
}

// SetInt adds an integer parameter unless it is zero.
func (p Params) SetInt(key string, value int) {
	if value != 0 {
		url.Values(p).Set(key, strconv.Itoa(value))
	}
}

// SetBool adds a boolean parameter when value is non-nil.
func (p Params) SetBool(key string, value *bool) {
	if value != nil {
		url.Values(p).Set(key, strconv.FormatBool(*value))
	}
}

// Values returns the underlying url.Values.
func (p Params) Values() url.Values { return url.Values(p) }
