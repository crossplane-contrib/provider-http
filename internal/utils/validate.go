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

// IsRequestValid checks if an HTTP request is valid.
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
// The allowedStatusCodes parameter specifies status codes that should not be treated as errors.
func IsHTTPError(statusCode int, allowedStatusCodes []int) bool {
	if statusCode < 400 || statusCode >= 600 {
		return false
	}

	// Check if this status code is in the allowed list
	for _, allowed := range allowedStatusCodes {
		if statusCode == allowed {
			return false
		}
	}

	return true
}

func IsUrlValid(input string) bool {
	u, err := url.ParseRequestURI(input)
	return err == nil && u.Scheme != "" && u.Host != ""
}
