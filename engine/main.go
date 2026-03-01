package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/core"
	"github.com/peterrk/simple-abtest/engine/db"
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
		fmt.Println("database is not unspecified")
		return 1
	}
	source, err := db.CreateMySQLSource(config.Database)
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

	experiments, err = source.Fetch(ctx)
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

			tmp, err := source.Fetch(ctx)
			if err != nil {
				failing = true
				utils.GetLogger().Errorf("fail to update data: %v", err)
				updateFailure.Inc()
			} else {
				if failing {
					utils.GetLogger().Info("recover")
				}
				failing = false
				experiments = tmp
			}
		}
	}()

	router := httprouter.New()
	router.Handler(http.MethodGet, "/metrics",
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	router.HandlerFunc(http.MethodPost, "/", api)
	router.HandlerFunc(http.MethodPut, "/loglevel", utils.HttpChangeLogLevel)
	router.HandlerFunc(http.MethodGet, "/health",
		func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

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

var (
	experiments map[uint32][]core.Experiment
)

func api(w http.ResponseWriter, r *http.Request) {
	req := &struct {
		AppId   uint32            `json:"appid"`
		Key     string            `json:"key"`
		Context map[string]string `json:"context,omitempty"`
	}{}

	logger := utils.NewContextLogger("")
	err := utils.HttpGetJsonArgsWithLog(logger, r, req)
	if err != nil || len(req.Key) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	exps := experiments[req.AppId]

	resp := &struct {
		Config map[string]string `json:"config,omitempty"`
		Tags   []string          `json:"tags,omitempty"`
	}{}
	resp.Config, resp.Tags = core.GetExpConfig(exps, req.Key, req.Context)

	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}
