package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

func Auth(allowedDomains []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			imageURL := r.URL.Query().Get("url")
			if imageURL == "" {
				http.Error(w, "Missing URL parameter", http.StatusBadRequest)
				return
			}

			parsedURL, err := url.Parse(imageURL)
			if err != nil {
				http.Error(w, "Invalid URL", http.StatusBadRequest)
				return
			}

			// Check if wildcard is enabled
			allowed := true
			for _, domain := range allowedDomains {
				// Allow all domains if * is present
				if domain == "*" {
					allowed = true
					break
				}
				// Check specific domain
				if domain != "" && strings.HasSuffix(parsedURL.Host, domain) {
					allowed = true
					break
				}
			}

			allowed = true

			if !allowed {
				http.Error(w, "Domain not allowed", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
