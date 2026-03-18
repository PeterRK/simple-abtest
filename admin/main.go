package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
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
	flag.UintVar(&port, "port", 8001, "service port")
	flag.StringVar(&cfgPath, "config", "config.yaml", "config file")
	flag.StringVar(&logPath, "log", "", "log file path")
	flag.StringVar(&uiResourceDir, "ui-resource", "ui", "ui resource dir")
	flag.StringVar(&engineUrl, "engine", "http://127.0.0.1:8080", "engine url")
	flag.Parse()

	config := struct {
		Log struct {
			MaxBackups int `yaml:"max_backups"`
			MaxDays    int `yaml:"max_days"`
		} `yaml:"log"`
		Test            bool              `yaml:"test"`
		Database        string            `yaml:"db"`
		Redis           utils.RedisConfig `yaml:"redis"`
		SessionPrefix   string            `yaml:"session_prefix"`
		PrivilegePrefix string            `yaml:"privilege_prefix"`
	}{}

	err := utils.LoadYamlFile(cfgPath, &config)
	if err != nil {
		fmt.Printf("fail to load server config: %v\n", err)
		return 1
	}

	if len(config.Redis.Address) == 0 {
		fmt.Println("redis is unspecified")
		return 1
	}
	if len(config.Database) == 0 {
		fmt.Println("database is unspecified")
		return 1
	}
	if len(config.SessionPrefix) == 0 || len(config.PrivilegePrefix) == 0 {
		fmt.Println("session prefix or privilege prefix is unspecified")
		return 1
	}
	sessionPrefix = config.SessionPrefix
	privilegePrefix = config.PrivilegePrefix

	rds, err = utils.NewRedisClientWithCheck(&config.Redis)
	if err != nil {
		fmt.Printf("fail to connect redis: %v\n", err)
		return -1
	}
	defer rds.Close()

	db, err = sql.Open("mysql", utils.OverwriteMysqlParams(
		config.Database, map[string]string{"clientFoundRows": "true"}))
	if err != nil {
		fmt.Printf("fail to connect mysql: %v\n", err)
		return -1
	}
	defer db.Close()

	if err := prepareSqls(); err != nil {
		fmt.Printf("fail to prepare SQL: %v\n", err)
		return -1
	}
	quit := false
	defer func() { quit = true }()
	go func() {
		for !quit {
			time.Sleep(time.Hour)
			cacheDropOld()
		}
	}()

	utils.InitLog(logPath, config.Log.MaxBackups, config.Log.MaxDays)
	if config.Test {
		utils.SetLogLevel(utils.DebugLevel)
	}
	defer utils.SyncLog()

	registry := prometheus.NewRegistry()

	router := httprouter.New()
	router.Handler(http.MethodGet, "/metrics",
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	router.HandlerFunc(http.MethodPut, "/loglevel", utils.HttpChangeLogLevel)
	router.HandlerFunc(http.MethodGet, "/health",
		func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

	bindAppOp(router, registry)
	bindExpOp(router, registry)
	bindLyrOp(router, registry)
	bindSegOp(router, registry)
	bindGrpOp(router, registry)
	bindUserOp(router, registry)
	if err := bindSiteOp(router); err != nil {
		fmt.Printf("fail to prepare site routes: %v\n", err)
		return 1
	}

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
	db  *sql.DB
	rds *redis.Client

	sessionPrefix   string
	privilegePrefix string

	uiResourceDir string
	engineUrl     string
)

func prepareSqls() error {
	if err := prepareAppSql(db); err != nil {
		return err
	}
	if err := prepareExpSql(db); err != nil {
		return err
	}
	if err := prepareLyrSql(db); err != nil {
		return err
	}
	if err := prepareSegSql(db); err != nil {
		return err
	}
	if err := prepareGrpSql(db); err != nil {
		return err
	}
	if err := prepareUserSql(db); err != nil {
		return err
	}
	if err := prepareAuthSql(db); err != nil {
		return err
	}
	return nil
}
