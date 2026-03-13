package main

import (
	"logger/config"
	"logger/logmsg"
	"sync"
)

func main() {
	cfg, err := config.Load("./logger.json")
	if err != nil {
		panic(err)
	}

	system,closers,err:=config.Build(cfg)
	if err != nil {
		panic(err)
	}

	defer func() {
		for _, c := range closers {
			c()
		}
	}()

	defer system.Shutdown()

	// ---- TEST LOGGING WITH STRUCTURED FIELDS ----
	for i := 0; i < 10000000; i++ {
		system.Info(
			"user request processed",
			logmsg.M("user_id", i),
			logmsg.M("endpoint", "/api/login"),
			logmsg.M("latency_ms", float64(i%100)),
			logmsg.M("success", i%2==0),
		)
	}

	system.Debug(
		"Debug message: application starting",
		logmsg.M("version", "1.0.0"),
		logmsg.M("env", "dev"),
		logmsg.M("debug_mode", true),
	)

	system.Info(
		"Info message: server initialized",
		logmsg.M("host", "localhost"),
		logmsg.M("port", 8080),
		logmsg.M("startup_time", 1.23),
	)

	system.Warning(
		"Warning message: memory usage high",
		logmsg.M("memory_mb", 2048),
		logmsg.M("threshold_mb", 1024),
		logmsg.M("usage_percent", 87.5),
	)

	system.Error(
		"Error message: database connection failed",
		logmsg.M("db", "users_db"),
		logmsg.M("host", "db-server"),
		logmsg.M("retry_count", 3),
		logmsg.M("critical", true),
	)

	system.Debug(
		"Debug message: processing request",
		logmsg.M("method", "GET"),
		logmsg.M("endpoint", "/api/users"),
		logmsg.M("request_id", 12345),
	)

	system.Info(
		"Info message: request completed",
		logmsg.M("status_code", 200),
		logmsg.M("latency_ms", 12.45),
		logmsg.M("client_ip", "192.168.1.20"),
	)

	system.Warning(
		"Warning message: cache miss",
		logmsg.M("cache_key", "user_profile_42"),
		logmsg.M("cache_layer", "redis"),
		logmsg.M("fallback_db", true),
	)

	system.Error(
		"Error message: unable to write to disk",
		logmsg.M("file", "/var/log/app.log"),
		logmsg.M("disk_usage_percent", 95),
		logmsg.M("retrying", true),
	)

	var wg sync.WaitGroup
	for i:=0;i<10;i++{
		wg.Add(1)
		go func(id int){
			defer wg.Done()
			for j:=0;j<100000000;j++{
				system.Error(
					"worker log",
					logmsg.M("worker",id),
					logmsg.M("iteration",j),
				)
			}
		}(i)
	}

	wg.Wait()

}