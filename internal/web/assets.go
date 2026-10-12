package web

import (
	_ "embed"
	"net/http"
)

//go:embed assets/logo.svg
var logoSVG []byte

func serveLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(logoSVG)
}
