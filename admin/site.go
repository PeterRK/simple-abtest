package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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
	defer r.Body.Close()
	uid, ok := verifySession(ctx, w, r)
	if !ok {
		return
	}
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

	if shouldServeGzip(r, name) {
		return serveUiFileGzip(w, r, localName, filepath.Base(localName), info.ModTime(), file)
	}

	http.ServeContent(w, r, filepath.Base(localName), info.ModTime(), file)
	return nil
}

func shouldServeGzip(r *http.Request, name string) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if len(r.Header.Get("Range")) != 0 {
		return false
	}
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	switch path.Ext(name) {
	case ".css", ".html", ".js", ".json", ".map", ".svg", ".txt":
		return true
	default:
		return false
	}
}

func serveUiFileGzip(
	w http.ResponseWriter, r *http.Request,
	cacheKey, name string, modTime time.Time, file *os.File,
) error {
	if data, ok := uiGzipCacheGet(cacheKey); ok {
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, r, name, modTime, bytes.NewReader(data))
		return nil
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return err
	}
	if _, err := zipper.Write(raw); err != nil {
		zipper.Close()
		return err
	}
	if err := zipper.Close(); err != nil {
		return err
	}

	uiGzipCacheSet(cacheKey, buf.Bytes())
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("Content-Encoding", "gzip")
	http.ServeContent(w, r, name, modTime, bytes.NewReader(buf.Bytes()))
	return nil
}

type uiGzipCacheStore struct {
	sync.RWMutex
	items      map[string][]byte
	totalBytes int64
}

const uiGzipCacheMaxBytes int64 = 32 << 20

var uiGzipCache = uiGzipCacheStore{
	items: make(map[string][]byte),
}

func uiGzipCacheGet(key string) ([]byte, bool) {
	uiGzipCache.RLock()
	data, ok := uiGzipCache.items[key]
	uiGzipCache.RUnlock()
	return data, ok
}

func uiGzipCacheSet(key string, data []byte) {
	if int64(len(data)) > uiGzipCacheMaxBytes {
		return
	}

	cloned := append([]byte(nil), data...)

	uiGzipCache.Lock()
	defer uiGzipCache.Unlock()

	if entry, ok := uiGzipCache.items[key]; ok {
		uiGzipCache.totalBytes -= int64(len(entry))
	}
	if uiGzipCache.totalBytes+int64(len(cloned)) > uiGzipCacheMaxBytes {
		uiGzipCache.items = make(map[string][]byte)
		uiGzipCache.totalBytes = 0
	}
	uiGzipCache.items[key] = cloned
	uiGzipCache.totalBytes += int64(len(cloned))
}
