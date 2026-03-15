package main

import (
	mlog "logger/start"
	"sync"
)

func main() {
	system,closers,err:=mlog.Run("./logger.json")
	if err!=nil{
		panic(err)
	}

	defer mlog.ShutDown(closers,system)


	// ---- TEST LOGGING WITH STRUCTURED FIELDS ----
	var wg sync.WaitGroup
	for i:=0;i<10;i++{
		wg.Add(1)
		go func(id int){
			defer wg.Done()
			for j:=0;j<10000;j++{
				system.Error(
					"worker log",
					mlog.M("worker",id),
					mlog.M("iteration",j),
				)
			}
		}(i)
	}


	system.Debug(
		"Debug message: application starting",
		mlog.M("version", "1.0.0"),
		mlog.M("env", "dev"),
		mlog.M("debug_mode", true),
	)

	system.Info(
		"Info message: server initialized",
		mlog.M("port", 8080),
		mlog.M("host", "localhost"),
		mlog.M("startup_time", 1.23),
	)

	system.Warning(
		"Warning message: memory usage high",
		mlog.M("memory_mb", 2048),
		mlog.M("threshold_mb", 1024),
		mlog.M("usage_percent", 87.5),
	)

	system.Error(
		"Error message: database connection failed",
		mlog.M("db", "users_db"),
		mlog.M("host", "db-server"),
		mlog.M("retry_count", 3),
		mlog.M("critical", true),
	)

	system.Debug(
		"Debug message: processing request",
		mlog.M("method", "GET"),
		mlog.M("endpoint", "/api/users"),
		mlog.M("request_id", 12345),
	)

	system.Info(
		"Info message: request completed",
		mlog.M("status_code", 200),
		mlog.M("latency_ms", 12.45),
		mlog.M("client_ip", "192.168.1.20"),
	)

	system.Warning(
		"Warning message: cache miss",
		mlog.M("cache_key", "user_profile_42"),
		mlog.M("cache_layer", "redis"),
		mlog.M("fallback_db", true),
	)

	system.Error(
		"Error message: unable to write to disk",
		mlog.M("file", "/var/log/app.log"),
		mlog.M("disk_usage_percent", 95),
		mlog.M("retrying", true),
	)


	wg.Wait()

}