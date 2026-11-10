package ui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetsAreVersionedAndCacheable(t *testing.T) {
	url := Asset("app.css")
	require.Regexp(t, `^/static/app\.css\?v=[0-9a-f]{8}$`, url)

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Cache-Control"), "immutable")

	w = httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}

// The CSP forbids unsafe-eval, so the Alpine build must be the CSP one and
// the page script must not use eval-like APIs.
func TestScriptsAvoidEval(t *testing.T) {
	for _, name := range []string{"alpine.csp.min.js", "app.js"} {
		b, err := fs.ReadFile(FS(), name)
		require.NoError(t, err)
		assert.False(t, strings.Contains(string(b), "new Function("), name)
	}
}
