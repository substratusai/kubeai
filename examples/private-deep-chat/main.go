package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// Run a web server that serves static content and proxies inference
// requests to KubeAI.
// Control access with basic auth.
func main() {
	kubeAIURL, err := url.Parse(os.Getenv("KUBEAI_ADDR"))
	if err != nil {
		log.Fatalf("failed to parse KubeAI address: %v", err)
	}

	staticHandler := http.FileServer(http.Dir("static"))
	proxyHandler := httputil.NewSingleHostReverseProxy(kubeAIURL)

	http.Handle("/", authUser(staticHandler))
	http.Handle("/openai/", authUserToKubeAI(proxyHandler)) //authUserToKubeAI(proxyHandler))

	listenAddr := os.Getenv("LISTEN_ADDR")
	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func authUser(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if _, matches := authenticate(user, pass); !ok || !matches {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func authUserToKubeAI(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, basicAuthProvided := r.BasicAuth()

		tenancy, authenticated := authenticate(user, pass)

		log.Printf("%s: %s - authenticating: basicAuthProvided=%t, user=%q, pass=%q, tenancy=%q, authenticated=%t",
			r.Method, r.URL.Path,
			basicAuthProvided,
			user, pass, strings.Join(tenancy, ","),
			authenticated,
		)

		if !basicAuthProvided || !authenticated || len(tenancy) == 0 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r.Header.Set("X-Label-Selector", fmt.Sprintf("tenancy in (%s)",
			strings.Join(tenancy, ","),
		))

		h.ServeHTTP(w, r)
	})
}

// authenticate checks the provided username and password.
// If the user is authenticated, it returns the tenancy groups the user belongs to.
func authenticate(user, pass string) ([]string, bool) {
	// In a real application, this would be a database lookup.
	userTable := map[string]struct {
		password string
		tenancy  []string
	}{
		"nick": {"nickspass", []string{"group-a"}},
		"sam":  {"samspass", []string{"group-b"}},
		"joe":  {"joespass", []string{"group-a", "group-b"}},
	}

	row, ok := userTable[user]
	if !ok {
		return nil, false
	}
	if row.password != pass {
		return nil, false
	}

	return row.tenancy, true
}
