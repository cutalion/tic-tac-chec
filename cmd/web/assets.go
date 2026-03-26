package main

import (
	"io"
	"net/http"
	"strings"
)

const assetVersionPlaceholder = "__ASSET_VERSION__"

func writeTemplatedAsset(w http.ResponseWriter, contentType string, raw []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	content := strings.ReplaceAll(string(raw), assetVersionPlaceholder, assetVersion)
	_, _ = io.WriteString(w, content)
}
