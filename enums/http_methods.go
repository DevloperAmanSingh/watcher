package enums

import (
	"fmt"
	"net/http"
	"strings"
)

type HttpMethod string

const (
	Get    HttpMethod = "get"
	Post   HttpMethod = "post"
	Patch  HttpMethod = "patch"
	Delete HttpMethod = "delete"
	Put    HttpMethod = "put"
)

func (hm HttpMethod) ToMethod() string {
	switch hm {
	case Get:
		return http.MethodGet
	case Post:
		return http.MethodPost
	case Patch:

		return http.MethodPatch
	case Put:
		return http.MethodPut
	case Delete:
		return http.MethodDelete
	default:
		return ""
	}
}

func (hm HttpMethod) ToString() string {
	switch hm {
	case Get:
		return "get"
	case Post:
		return "post"
	case Patch:

		return "patch"
	case Put:
		return "put"
	case Delete:
		return "delete"
	default:
		return ""
	}
}

func ParseHttpMethod(s string) (HttpMethod, error) {
	switch strings.ToLower(s) {
	case "get":
		return Get, nil
	case "post":
		return Post, nil
	case "patch":
		return Patch, nil
	case "delete":
		return Delete, nil
	case "put":
		return Put, nil
	default:
		return "", fmt.Errorf("invalid http method: %s", s)
	}
}
