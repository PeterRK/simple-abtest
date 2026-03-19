package main

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
)

func bindSiteOp(router *httprouter.Router) error {
	engine, err := url.Parse(engineUrl)
	if err != nil {
		return err
	}

	router.Handle(http.MethodPost, "/engine", newEngineProxy(engine))
	router.Handle(http.MethodGet, "/ui/*filepath", handleUi)
	router.Handle(http.MethodHead, "/ui/*filepath", handleUi)
	return nil
}

func newEngineProxy(target *url.URL) httprouter.Handle {
	proxy := httputil.NewSingleHostReverseProxy(target)
	originDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originDirector(req)
		req.URL.Path = joinUrlPath(target.Path, "/")
		req.URL.RawPath = ""
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusBadGateway)
	}
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(r.Context(), "engineProxy")
		if _, ok := verifySession(ctx, w, r); !ok {
			return
		}
		proxy.ServeHTTP(w, r)
	}
}

func joinUrlPath(basePath, subPath string) string {
	switch {
	case basePath == "":
		return subPath
	case subPath == "":
		return basePath
	default:
		return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(subPath, "/")
	}
}

func handleUi(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	name := path.Clean("/" + strings.TrimPrefix(p.ByName("filepath"), "/"))
	if name == "/" {
		name = "/index.html"
	}

	err := serveUiFile(w, r, name)
	if err == nil {
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if path.Ext(name) != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := serveUiFile(w, r, "/index.html"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func serveUiFile(w http.ResponseWriter, r *http.Request, name string) error {
	localName := filepath.Join(uiResourceDir, filepath.FromSlash(strings.TrimPrefix(name, "/")))
	file, err := os.Open(localName)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.ErrNotExist
	}

	http.ServeContent(w, r, filepath.Base(localName), info.ModTime(), file)
	return nil
}
