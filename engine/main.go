package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/core"
	"github.com/peterrk/simple-abtest/engine/data"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	os.Exit(Main())
}

func Main() int {
	var (
		port    uint
		cfgPath string
		logPath string
	)
	flag.UintVar(&port, "port", 8080, "service port")
	flag.StringVar(&cfgPath, "config", "config.yaml", "config file")
	flag.StringVar(&logPath, "log", "", "log file path")
	flag.Parse()

	config := struct {
		Log struct {
			MaxBackups int `yaml:"max_backups"`
			MaxDays    int `yaml:"max_days"`
		} `yaml:"log"`
		Test      bool   `yaml:"test"`
		Database  string `yaml:"db"`
		IntervalS uint32 `yaml:"interval_s"`
	}{}

	err := utils.LoadYamlFile(cfgPath, &config)
	if err != nil {
		fmt.Printf("fail to load server config: %v\n", err)
		return 1
	}

	if len(config.Database) == 0 {
		fmt.Println("database is unspecified")
		return 1
	}
	source, err := data.CreateMySQLSource(config.Database)
	if err != nil {
		fmt.Printf("fail to prepare data source: %v\n", err)
		return 1
	}
	defer source.Close()

	utils.InitLog(logPath, config.Log.MaxBackups, config.Log.MaxDays)
	if config.Test {
		utils.SetLogLevel(utils.DebugLevel)
	}
	defer utils.SyncLog()

	updateFailure := utils.NewCounter("update_failure")
	registry := prometheus.NewRegistry()
	registry.MustRegister(updateFailure)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetch := func() (map[uint32]*Application, error) {
		exps, err := source.Fetch(ctx)
		if err != nil {
			return nil, err
		}
		apps := make(map[uint32]*Application, len(exps))
		for k, v := range exps {
			pack, err := toJsonAndZip(&v.Experiments)
			if err != nil {
				return nil, err
			}
			apps[k] = &Application{
				Application: v,
				pack:        pack,
			}
		}
		return apps, nil
	}

	applications, err = fetch()
	if err != nil {
		fmt.Printf("fail to get data: %v\n", err)
		return -1
	}

	if config.IntervalS == 0 {
		config.IntervalS = 300
	}

	go func() {
		failing := false
		for ctx.Err() == nil {
			time.Sleep(time.Second * time.Duration(config.IntervalS))

			apps, err := fetch()
			if err != nil {
				failing = true
				utils.GetLogger().Errorf("fail to update data: %v", err)
				updateFailure.Inc()
			} else {
				if failing {
					utils.GetLogger().Info("recover")
				}
				failing = false
				applications = apps
			}
		}
	}()

	router := httprouter.New()
	router.Handler(http.MethodGet, "/metrics",
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	router.HandlerFunc(http.MethodPut, "/loglevel", utils.HttpChangeLogLevel)
	router.HandlerFunc(http.MethodGet, "/health",
		func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

	router.Handle(http.MethodPost, "/", abtest)
	router.Handle(http.MethodGet, "/app/:id", fetchAppInfo)

	if config.Test {
		router.HandlerFunc(http.MethodGet, "/debug/pprof/", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/cmdline", pprof.Cmdline)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/profile", pprof.Profile)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/symbol", pprof.Symbol)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/trace", pprof.Trace)
	}

	utils.GetLogger().Info("Server Up")
	if err := utils.RunHttpServer(router, fmt.Sprintf(":%d", port)); err != nil {
		return -1
	}
	utils.GetLogger().Info("Server Down")
	return 0
}

func toJsonAndZip(obj any) ([]byte, error) {
	var buf bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(zipper).Encode(obj); err != nil {
		_ = zipper.Close()
		return nil, err
	}
	if err := zipper.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type Application struct {
	data.Application
	pack []byte // gzipped
}

var (
	applications map[uint32]*Application
)

func abtest(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	req := &struct {
		AppId   uint32            `json:"appid"`
		Key     string            `json:"key"`
		Context map[string]string `json:"context,omitempty"`
	}{}

	logger := utils.NewContextLogger("abtest")
	err := utils.HttpGetJsonArgsWithLog(logger, r, req)
	if err != nil || len(req.Key) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	app := applications[req.AppId]
	if app == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if app.AccessToken != r.Header.Get("ACCESS_TOKEN") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	resp := &struct {
		Config map[string]string `json:"config,omitempty"`
		Tags   []string          `json:"tags,omitempty"`
	}{}
	resp.Config, resp.Tags = core.GetExpConfig(app.Experiments, req.Key, req.Context)

	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func fetchAppInfo(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	app := applications[uint32(id)]
	if app == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if app.AccessToken != r.Header.Get("ACCESS_TOKEN") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	w.Write(app.pack)
}
