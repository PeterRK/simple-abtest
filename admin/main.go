package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
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
	flag.Parse()

	config := struct {
		Log struct {
			MaxBackups int `yaml:"max_backups"`
			MaxDays    int `yaml:"max_days"`
		} `yaml:"log"`
		Test        bool              `yaml:"test"`
		Database    string            `yaml:"db"`
		Redis       utils.RedisConfig `yaml:"redis"`
		RedisPrefix string            `yaml:"redis_prefix"`
		Secret      string            `yaml:"secret"`
		Engine      string            `yaml:"engine"`
		Auth        struct {
			SessionTTLMinutes        int `yaml:"session_ttl_minutes"`
			PrivilegeCacheTTLMinutes int `yaml:"privilege_cache_ttl_minutes"`
			RelationCacheTTLDays     int `yaml:"relation_cache_ttl_days"`
		} `yaml:"auth"`
		RateLimit struct {
			Login struct {
				Limit         int `yaml:"limit"`
				WindowMinutes int `yaml:"window_minutes"`
			} `yaml:"login"`
			UserUpdate struct {
				Limit         int `yaml:"limit"`
				WindowMinutes int `yaml:"window_minutes"`
			} `yaml:"user_update"`
			UserDelete struct {
				Limit         int `yaml:"limit"`
				WindowMinutes int `yaml:"window_minutes"`
			} `yaml:"user_delete"`
		} `yaml:"rate_limit"`
	}{}

	err := utils.LoadYamlFile(cfgPath, &config)
	if err != nil {
		fmt.Printf("fail to load server config: %v\n", err)
		return 1
	}
	predefinedSecret = config.Secret
	engineUrl = config.Engine
	if len(engineUrl) == 0 {
		engineUrl = "http://127.0.0.1:8080"
	}

	if config.Auth.SessionTTLMinutes > 0 &&
		config.Auth.SessionTTLMinutes <= 7*24*60 {
		sessionTTL = time.Duration(config.Auth.SessionTTLMinutes) * time.Minute
	}
	if config.Auth.PrivilegeCacheTTLMinutes > 0 &&
		config.Auth.PrivilegeCacheTTLMinutes <= 240 {
		privCacheTTL = time.Duration(config.Auth.PrivilegeCacheTTLMinutes) * time.Minute
	}
	if config.Auth.RelationCacheTTLDays > 0 &&
		config.Auth.RelationCacheTTLDays <= 90 {
		relationTTL = uint32(config.Auth.RelationCacheTTLDays * 24 * 60 * 60)
	}

	applyRateLimitRuleConfig := func(rule *rateLimitRule, limit, windowMinutes int) {
		if limit > 0 && limit <= 1000 &&
			windowMinutes > 0 && windowMinutes <= 24*60 {
			rule.limit = int64(limit)
			rule.window = time.Duration(windowMinutes) * time.Minute
		}
	}
	applyRateLimitRuleConfig(&loginRateLimitByAccount,
		config.RateLimit.Login.Limit, config.RateLimit.Login.WindowMinutes)
	applyRateLimitRuleConfig(&updateRateLimitByUser,
		config.RateLimit.UserUpdate.Limit, config.RateLimit.UserUpdate.WindowMinutes)
	applyRateLimitRuleConfig(&deleteRateLimitByUser,
		config.RateLimit.UserDelete.Limit, config.RateLimit.UserDelete.WindowMinutes)

	if len(config.Redis.Address) == 0 {
		fmt.Println("redis is unspecified")
		return 1
	}
	if len(config.Database) == 0 {
		fmt.Println("database is unspecified")
		return 1
	}
	redisPrefix = config.RedisPrefix

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

	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, "/health",
		func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

	bindAppOp(router)
	bindExpOp(router)
	bindLyrOp(router)
	bindSegOp(router)
	bindGrpOp(router)
	bindUserOp(router)
	bindResultOp(router)
	if err := bindSiteOp(router); err != nil {
		fmt.Printf("fail to prepare site routes: %v\n", err)
		return 1
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

	redisPrefix string

	uiResourceDir string
	engineUrl     string

	predefinedSecret string
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
	if err := prepareResultSql(db); err != nil {
		return err
	}
	if err := prepareAuthSql(db); err != nil {
		return err
	}
	return nil
}
