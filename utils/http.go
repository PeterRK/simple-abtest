package utils

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	json "github.com/goccy/go-json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func httpGetJsonArgs(r *http.Request, obj any, log bool) error {
	if log {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		if logger := GetLogger(); logger != nil {
			logger.Debug(UnsafeBytesToString(raw))
		}
		return json.Unmarshal(raw, obj)
	}
	defer io.Copy(io.Discard, r.Body)
	return json.NewDecoder(r.Body).Decode(obj)
}

func httpReplyJson(w http.ResponseWriter, code int, obj any, log bool) error {
	if obj == nil {
		w.WriteHeader(code)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if log {
		raw, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		if logger := GetLogger(); logger != nil {
			logger.Debug(UnsafeBytesToString(raw))
		}
		_, err = w.Write(raw)
		return err
	}
	return json.NewEncoder(w).Encode(obj)
}

func HttpGetJsonArgs(r *http.Request, obj any) error {
	return httpGetJsonArgs(r, obj, false)
}

func HttpGetJsonArgsWithLog(r *http.Request, obj any) error {
	return httpGetJsonArgs(r, obj, true)
}

func HttpReplyJson(w http.ResponseWriter, code int, obj any) error {
	return httpReplyJson(w, code, obj, false)
}

func HttpReplyJsonWithLog(w http.ResponseWriter, code int, obj any) error {
	return httpReplyJson(w, code, obj, true)
}

func HttpGetJsonArgsWithLogger(logger LogCtx, r *http.Request, obj any) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	logger.LogDebug(UnsafeBytesToString(raw))
	return json.Unmarshal(raw, obj)
}

func HttpReplyJsonWithLogger(logger LogCtx, w http.ResponseWriter, code int, obj any) error {
	if obj == nil {
		w.WriteHeader(code)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	logger.LogDebug(UnsafeBytesToString(raw))
	_, err = w.Write(raw)
	return err
}

var (
	pbJsonMarshaller   = protojson.MarshalOptions{UseProtoNames: true}
	pbJsonUnmarshaller = protojson.UnmarshalOptions{DiscardUnknown: true}
)

func HttpGetPbArgs(r *http.Request, m proto.Message) (bin bool, err error) {
	bin = r.Header.Get("Content-Type") != "application/json"
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return bin, err
	}
	if bin {
		err = proto.Unmarshal(raw, m)
	} else {
		err = pbJsonUnmarshaller.Unmarshal(raw, m)
	}
	return bin, err
}

func HttpReplyPb(w http.ResponseWriter, code int, m proto.Message, bin bool) (err error) {
	if m == nil {
		w.WriteHeader(code)
		return nil
	}
	var raw []byte
	if bin {
		w.Header().Set("Content-Type", "application/protobuf")
		raw, err = proto.Marshal(m)
	} else {
		w.Header().Set("Content-Type", "application/json")
		raw, err = pbJsonMarshaller.Marshal(m)
	}
	if err != nil {
		return err
	}
	w.WriteHeader(code)
	_, err = w.Write(raw)
	return err
}

func RunHttpServer(router http.Handler, address string) error {
	srv := &http.Server{
		Addr:    address,
		Handler: router,
	}

	signal.Ignore(syscall.SIGPIPE)
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		if err := srv.Shutdown(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "!!!", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "!!!!", err)
		return err
	}
	return nil
}

func NewRestfulRequest(ctx context.Context, method, url string,
	headers, params map[string]string, obj any) (*http.Request, error) {
	var body io.Reader
	sendRaw := false
	if obj != nil {
		if u, _ := obj.(*[]byte); u != nil {
			sendRaw = true
			body = bytes.NewBuffer(*u)
		} else {
			buf := &bytes.Buffer{}
			if err := json.NewEncoder(buf).Encode(obj); err != nil {
				return nil, err
			}
			body = buf
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	if len(params) != 0 {
		query := req.URL.Query()
		for k, v := range params {
			query.Set(k, v)
		}
		req.URL.RawQuery = query.Encode()
	}

	if sendRaw {
		req.Header.Set("Content-Type", "application/octet-stream")
	} else if obj != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

type responseError struct {
	msg string
}

func (e *responseError) Error() string {
	return e.msg
}

func newResponseError(code int, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	respErr := &responseError{}
	if utf8.Valid(data) {
		respErr.msg = fmt.Sprintf("[%d] %s", code, UnsafeBytesToString(data))
	} else {
		respErr.msg = fmt.Sprintf("[%d]:%s", code, hex.Dump(data))
	}
	return respErr
}

func HandleRestfulResponse(resp *http.Response, obj any) (code int, err error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated &&
		!(resp.StatusCode == http.StatusNoContent && obj == nil) {
		return resp.StatusCode, newResponseError(resp.StatusCode, resp.Body)
	}
	defer resp.Body.Close()
	if obj == nil {
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	if u, _ := obj.(*[]byte); u != nil {
		*u, err = io.ReadAll(resp.Body)
	} else {
		defer io.Copy(io.Discard, resp.Body)
		err = json.NewDecoder(resp.Body).Decode(obj)
	}
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

var defaultDialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

func CustomizeDefaultHttpClient(maxIdleConns, maxIdleConnsPerHost, maxConnsPerHost int) {
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           defaultDialer.DialContext,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			MaxConnsPerHost:       maxConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: time.Second * 5,
	}
}

func RestfulDo(ctx context.Context, method, url string,
	headers, params map[string]string, obj, out any) (code int, err error) {
	req, err := NewRestfulRequest(ctx, method, url, headers, params, obj)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	return HandleRestfulResponse(resp, out)
}

func RestfulGet(ctx context.Context, url string,
	headers, params map[string]string, out any) (code int, err error) {
	return RestfulDo(ctx, http.MethodGet, url, headers, params, nil, out)
}

func RestfulDelete(ctx context.Context, url string,
	headers, params map[string]string) (code int, err error) {
	return RestfulDo(ctx, http.MethodDelete, url, headers, params, nil, nil)
}

func RestfulPost(ctx context.Context, url string,
	headers, params map[string]string, obj, out any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPost, url, headers, params, obj, out)
}

func RestfulPatch(ctx context.Context, url string,
	headers, params map[string]string, obj any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPatch, url, headers, params, obj, nil)
}

func RestfulPut(ctx context.Context, url string,
	headers, params map[string]string, obj any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPut, url, headers, params, obj, nil)
}
