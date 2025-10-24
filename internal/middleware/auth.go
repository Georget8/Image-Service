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

			// Decode URL-encoded characters (like %20 for space, %201 etc)
			decodedURL, err := url.QueryUnescape(imageURL)
			if err != nil {
				// If decode fails, use original URL
				decodedURL = imageURL
			}

			// Parse the URL
			parsedURL, err := url.Parse(decodedURL)
			if err != nil {
				// Try parsing the original URL if decoded fails
				parsedURL, err = url.Parse(imageURL)
				if err != nil {
					http.Error(w, "Invalid URL", http.StatusBadRequest)
					return
				}
			}

			// Check if wildcard is enabled
			allowed := false
			for _, domain := range allowedDomains {
				// Allow all domains if * is present
				if domain == "*" {
					allowed = true
					break
				}
				// Check specific domain (supports subdomains)
				if domain != "" && (strings.HasSuffix(parsedURL.Host, domain) || parsedURL.Host == domain) {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Domain not allowed", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
