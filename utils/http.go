package utils

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// HttpGetJsonArgs decodes a JSON request body into obj without logging.
func HttpGetJsonArgs(r *http.Request, obj any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(obj)
}

// HttpReplyJson writes obj as JSON response with the given HTTP status code.
func HttpReplyJson(w http.ResponseWriter, code int, obj any) error {
	if obj == nil {
		w.WriteHeader(code)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(obj)
}

// HttpGetJsonArgsWithLog decodes a JSON body into obj and logs the raw payload.
func HttpGetJsonArgsWithLog(logger *ContextLogger, r *http.Request, obj any) error {
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	logger.Debug(UnsafeBytesToString(raw))
	return json.Unmarshal(raw, obj)
}

// HttpReplyJsonWithLog writes obj as JSON and logs the raw payload.
func HttpReplyJsonWithLog(logger *ContextLogger, w http.ResponseWriter, code int, obj any) error {
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
	logger.Debug(UnsafeBytesToString(raw))
	_, err = w.Write(raw)
	return err
}

var (
	pbJsonMarshaller   = protojson.MarshalOptions{UseProtoNames: true}
	pbJsonUnmarshaller = protojson.UnmarshalOptions{DiscardUnknown: true}
)

// HttpGetPbArgs reads a protobuf request into m.
// When Content-Type is application/json it expects JSON, otherwise protobuf wire format.
func HttpGetPbArgs(r *http.Request, m proto.Message) (bin bool, err error) {
	defer r.Body.Close()
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	bin = contentType != "application/json"
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

// HttpReplyPb writes a protobuf message as either JSON or protobuf wire format.
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

// RunHttpServer starts an HTTP server on address and shuts it down on SIGINT/SIGTERM.
func RunHttpServer(router http.Handler, address string) error {
	srv := &http.Server{
		Addr:    address,
		Handler: router,
	}

	signal.Ignore(syscall.SIGPIPE)
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigs)
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

// NewRestfulRequest builds an HTTP request with optional JSON body and headers/params.
// If obj is *[]byte the raw bytes are sent as the request body.
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

func newResponseError(code int, data []byte) error {
	err := &responseError{}
	if utf8.Valid(data) {
		err.msg = fmt.Sprintf("[%d] %s", code, UnsafeBytesToString(data))
	} else {
		err.msg = fmt.Sprintf("[%d]:%s", code, hex.Dump(data))
	}
	return err
}

// HandleRestfulResponse decodes an HTTP response into obj.
// When obj is nil it discards the body; when obj is *[]byte it returns the raw body.
func HandleRestfulResponse(resp *http.Response, obj any) (code int, err error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated &&
		!(resp.StatusCode == http.StatusNoContent && obj == nil) {
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return resp.StatusCode, err
		}
		return resp.StatusCode, newResponseError(resp.StatusCode, data)
	}
	defer resp.Body.Close()
	if obj == nil {
		return resp.StatusCode, nil
	}
	if u, _ := obj.(*[]byte); u != nil {
		*u, err = io.ReadAll(resp.Body)
	} else {
		err = json.NewDecoder(resp.Body).Decode(obj)
	}
	if err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

var defaultDialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

// CustomizeDefaultHttpClient replaces http.DefaultClient with a client using
// the given connection pooling limits.
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

// RestfulDo sends an HTTP request and decodes the response into out.
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

// RestfulGet sends an HTTP GET request and decodes the response into out.
func RestfulGet(ctx context.Context, url string,
	headers, params map[string]string, out any) (code int, err error) {
	return RestfulDo(ctx, http.MethodGet, url, headers, params, nil, out)
}

// RestfulDelete sends an HTTP DELETE request and discards the response body.
func RestfulDelete(ctx context.Context, url string,
	headers, params map[string]string) (code int, err error) {
	return RestfulDo(ctx, http.MethodDelete, url, headers, params, nil, nil)
}

// RestfulPost sends an HTTP POST with an optional JSON body and decodes response into out.
func RestfulPost(ctx context.Context, url string,
	headers, params map[string]string, obj, out any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPost, url, headers, params, obj, out)
}

// RestfulPatch sends an HTTP PATCH with an optional JSON body.
func RestfulPatch(ctx context.Context, url string,
	headers, params map[string]string, obj any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPatch, url, headers, params, obj, nil)
}

// RestfulPut sends an HTTP PUT with an optional JSON body.
func RestfulPut(ctx context.Context, url string,
	headers, params map[string]string, obj any) (code int, err error) {
	return RestfulDo(ctx, http.MethodPut, url, headers, params, obj, nil)
}
