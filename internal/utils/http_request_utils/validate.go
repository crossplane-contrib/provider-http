package httprequest

import "errors"

const (
	errEmptyMethod = "no method is specified"
	errEmptyURL    = "no url is specified"
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
