package integration

import (
	"net/http"
	"testing"
)

func sessionTokenFromHeaders(t *testing.T, headers http.Header) string {
	t.Helper()
	for _, cookie := range (&http.Response{Header: headers}).Cookies() {
		if cookie.Name != "threaden_session" {
			continue
		}
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value == "" {
			t.Fatalf("insecure session cookie: %+v", cookie)
		}
		return cookie.Value
	}
	t.Fatal("session cookie missing")
	return ""
}
