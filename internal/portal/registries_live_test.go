//go:build live

package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveListRegistries(t *testing.T) {
	// Use real presets embed from repo FS
	presets := os.DirFS("..")
	// fix path - portal tests use relative
}
