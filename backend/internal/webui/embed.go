package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDist embed.FS

// DistFS returns an fs.FS rooted at the embedded production frontend build (dist/).
func DistFS() (fs.FS, error) {
	return fs.Sub(embeddedDist, "dist")
}
