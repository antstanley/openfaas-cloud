package handlers

import (
	"crypto"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// MakeQueryHandler returns whether a client can access a resource
func MakeQueryHandler(config *Config, protected []string) func(http.ResponseWriter, *http.Request) {
	keydata, err := ioutil.ReadFile(config.PublicKeyPath)
	if err != nil {
		log.Fatalf("unable to read path: %s, error: %s", config.PublicKeyPath, err.Error())
	}

	publicKey, keyErr := jwt.ParseECPublicKeyFromPEM(keydata)
	if keyErr != nil {
		log.Fatalf("unable to parse public key: %s", keyErr.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		resource := query.Get("r")

		status := http.StatusOK
		if len(resource) == 0 {
			status = http.StatusBadRequest
		} else if isProtected(resource, protected) {
			started := time.Now()
			cookieStatus := validCookie(r, cookieName, publicKey)
			log.Printf("Cookie verified: %fs [%d]", time.Since(started).Seconds(), cookieStatus)

			switch cookieStatus {
			case http.StatusOK:
				status = http.StatusOK
				break
			case http.StatusNetworkAuthenticationRequired:
				status = http.StatusTemporaryRedirect
				log.Printf("No cookie or an invalid cookie was found.\n")
				break
			default:
				log.Printf("A valid cookie was not found.\n")
				status = http.StatusUnauthorized
				break
			}
		}

		log.Printf("Validate %s => %d\n", resource, status)

		if status == http.StatusTemporaryRedirect {
			redirect := buildGitHubURL(config, "", config.Scope)
			log.Printf("Go to %s\n", redirect.String())

			http.Redirect(w, r, redirect.String(), http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(status)

	}
}

func isProtected(resource string, protected []string) bool {
	for _, prefix := range protected {
		if strings.HasPrefix(resource, prefix) {
			return true
		}
	}
	return false
}

func validCookie(r *http.Request, cookieName string, publicKey crypto.PublicKey) int {

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return http.StatusNetworkAuthenticationRequired
	}

	claims := OpenFaaSCloudClaims{}
	if len(cookie.Value) > 0 {
		log.Println("Cookie value: ", cookie.Value)

		parsed, parseErr := jwt.ParseWithClaims(cookie.Value, &claims, func(token *jwt.Token) (interface{}, error) {
			return publicKey, nil
		})

		if parseErr != nil {
			log.Println(parseErr)
			return http.StatusUnauthorized
		}

		if parsed.Valid {
			log.Println("Claims", claims)
			log.Printf("Validated JWT for (%s) %s", claims.Subject, claims.Name)

			return http.StatusOK
		}

	}

	return http.StatusUnauthorized
}