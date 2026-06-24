package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func IndexHTML() ([]byte, error) {
	return staticFiles.ReadFile("static/index.html")
}
