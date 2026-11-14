// Package web embeds the browser-based API tester served at "/".
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var content embed.FS

// Static returns the static site's file system rooted at the static directory.
func Static() fs.FS {
	sub, err := fs.Sub(content, "static")
	if err != nil {
		panic(err) // unreachable: the directory is embedded at compile time
	}
	return sub
}
