package reflector

import (
	"errors"
	"net/url"
)

var secrets = map[string]string{
	"secret": "secret.whistlerapp.org",
}

var allowed = map[string]bool{
	"whistlerapp.org":     true,
	"www.whistlerapp.org": true,
}

// GetWhistlerURL returns destination url Whistler reflector
// will forward to. Destination will be checked if it is allowed
// and source host will be translated if secret.
// TODO: if we go this way than we need to think how to replace
// all occurences of secret host in replies from secret server,
// or should we do that at all..
func GetWhistlerURL(rawurl string) (string, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}

	secretHost := secrets[url.Host]
	if secretHost != "" {
		url.Host = secretHost
	}

	if !isURLAllowed(url) {
		return "", errors.New("Forward URL not allowed")
	}

	return url.String(), nil
}

func isURLAllowed(url *url.URL) bool {
	if len(allowed) == 0 {
		return true
	}

	return allowed[url.Host]
}
