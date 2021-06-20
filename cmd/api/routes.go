package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	router.HandlerFunc(http.MethodGet, "/v1/albums", app.listAlbumsHandler)
	router.HandlerFunc(http.MethodPost, "/v1/albums", app.createAlbumsHandler)
	router.HandlerFunc(http.MethodGet, "/v1/albums/:id", app.showAlbumsHandler)
	router.HandlerFunc(http.MethodPut, "/v1/albums/:id", app.updateAlbumHandler)
	router.HandlerFunc(http.MethodPatch, "/v1/albums/:id", app.updateAlbumHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/albums/:id", app.deleteAlbumHandler)

	return app.recoverPanic(app.rateLimit(router))
}
