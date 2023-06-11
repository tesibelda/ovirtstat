// netplus is a basic net helper library
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package netplus

import (
	"errors"
	"fmt"
	"net/url"
)

var ErrorURLParsing = errors.New("error parsing URL")

// PaseURL parses an URL params
func PaseURL(anURL, user, pass string) (*url.URL, error) {
	u, err := url.Parse(anURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrorURLParsing.Error(), err)
	}
	if u == nil {
		return nil, fmt.Errorf("%w: returned nil", ErrorURLParsing)
	}
	u.User = url.UserPassword(user, pass)

	return u, nil
}
