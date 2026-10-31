// Package ui is the shared design system for every web surface: Tailwind
// tokens, Penguin UI style components (templ), and the embedded static assets
// (htmx, Alpine CSP build, lightweight-charts, app.js, built CSS).
//
// Regenerate with `make generate`; app.css and *_templ.go are committed.
package ui

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
)

//go:embed static
var files embed.FS

var static = func() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}()

// versions maps an asset name to a content hash so URLs can be cached forever.
var versions = func() map[string]string {
	out := map[string]string{}
	entries, err := fs.ReadDir(static, ".")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		b, err := fs.ReadFile(static, e.Name())
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(b)
		out[e.Name()] = hex.EncodeToString(sum[:4])
	}
	return out
}()

// FS exposes the embedded assets (the CSP test hashes lightweight-charts from it).
func FS() fs.FS { return static }

// Asset returns the cache-busted URL of an embedded file.
func Asset(name string) string {
	return "/static/" + name + "?v=" + versions[name]
}

// Handler serves /static/. Mount it on the "GET /static/" pattern.
func Handler() http.Handler {
	files := http.StripPrefix("/static/", http.FileServerFS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
