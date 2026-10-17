// Package web provides embedded static frontend assets.
package web

import "embed"

// Files contains the embedded web static assets (index.html, style.css, app.js).
//
//go:embed index.html style.css app.js
var Files embed.FS

// FS is an alias for Files.
var FS = Files
