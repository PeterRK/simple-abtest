package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/sign"
)

func bindSiteOp(router *httprouter.Router) error {
	router.Handle(http.MethodPost, "/engine", handleEngineProxy)
	router.Handle(http.MethodGet, "/", handleRootRedirect)
	router.Handle(http.MethodGet, "/ui/*filepath", handleUi)
	router.Handle(http.MethodHead, "/ui/*filepath", handleUi)
	return nil
}

func handleEngineProxy(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewContext(r.Context(), "engineProxy")
	uid, ok := verifySession(ctx, w, r)
	if !ok {
		return
	}

	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ctx.Errorf("fail to read engine proxy body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		AppId uint32 `json:"appid"`
	}{}
	if err := json.Unmarshal(raw, req); err != nil || req.AppId == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	allowed, err := checkAppPrivilege(ctx, uid, req.AppId, privilegeReadOnly)
	if err != nil {
		ctx.Errorf("fail to check engine proxy privilege: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !allowed {
		ctx.Debugf("engine proxy rejected uid=%d app=%d", uid, req.AppId)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var signingSecret string
	err = appSql.getToken.QueryRowContext(ctx, req.AppId).Scan(&signingSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[app.getToken]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	token := sign.BuildPublicToken(signingSecret, req.AppId, uint32(time.Now().Add(time.Minute).Unix()))
	proxyReq, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		engineUrl,
		bytes.NewReader(raw),
	)
	if err != nil {
		ctx.Errorf("fail to build engine proxy request: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("ACCESS_TOKEN", token)

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		ctx.Errorf("fail to call engine: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); len(ct) != 0 {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		ctx.Warnf("fail to copy engine response: %v", err)
	}
}

func handleRootRedirect(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.Redirect(w, r, "/ui/", http.StatusFound)
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
	if errors.Is(err, os.ErrPermission) {
		w.WriteHeader(http.StatusForbidden)
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
	baseDir, err := filepath.Abs(uiResourceDir)
	if err != nil {
		return err
	}

	localName, err := filepath.Abs(filepath.Join(baseDir,
		filepath.FromSlash(strings.TrimPrefix(name, "/"))))
	if err != nil {
		return err
	}

	// Keep the resolved file path within the configured UI resource directory.
	rel, err := filepath.Rel(baseDir, localName)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return os.ErrPermission
	}

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
