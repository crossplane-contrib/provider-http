package utils

import (
	"net/url"

	"github.com/pkg/errors"
)

const (
	errEmptyMethod = "no method is specified"
	ErrInvalidURL  = "invalid url %s"
	ErrStatusCode  = "HTTP %s request failed with status code: %s"
)

func IsRequestValid(method string, url string) error {
	if method == "" {
		return errors.New(errEmptyMethod)
	}

	if !IsUrlValid(url) {
		return errors.Errorf(ErrInvalidURL, url)
	}

	return nil
}

// IsHTTPSuccess checks if an HTTP status code indicates success.
func IsHTTPSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// IsHTTPError checks if an HTTP status code indicates an error.
func IsHTTPError(statusCode int) bool {
	return statusCode >= 400 && statusCode < 600
}

func IsUrlValid(input string) bool {
	u, err := url.ParseRequestURI(input)
	return err == nil && u.Scheme != "" && u.Host != ""
}
