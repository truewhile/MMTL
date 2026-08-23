package service

import (
	"net/url"
	"os"
	"strings"
)

func readLocalSTRMTarget(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a discovered .strm file under the configured library root.
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		candidate := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if candidate == "" || strings.HasPrefix(candidate, "#") {
			continue
		}
		if strings.HasPrefix(candidate, "/api/") || strings.HasPrefix(candidate, "/Videos/") || strings.HasPrefix(candidate, "/videos/") {
			return candidate, nil
		}
		u, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "webdav", "davs", "alist", "alists", "openlist", "openlists":
			return candidate, nil
		}
	}
	return "", nil
}
