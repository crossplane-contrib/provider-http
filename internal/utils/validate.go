package utils

import (
	"errors"
)

const (
	errEmptyMethod = "no method is specified"
	errEmptyURL    = "no url is specified"
	ErrStatusCode  = "HTTP request failed with status code: %s"
)

func IsRequestValid(method string, url string) error {
	if method == "" {
		return errors.New(errEmptyMethod)
	}

	if url == "" {
		return errors.New(errEmptyURL)
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
