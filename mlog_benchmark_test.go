package mlog

import (
	"path/filepath"
	"testing"
)

func benchmarkLogger(b *testing.B, level string) (*Logger, func()) {
	b.Helper()

	logPath := filepath.Join(b.TempDir(), "benchmark.log")
	return New(
		WithLevel(level),
		WithFile(logPath),
		WithBuffer(b.N+1),
		WithBatchSize(1024),
		WithFlushInterval(60*60*1000),
	)
}

func BenchmarkLoggerInfo(b *testing.B) {
	log, stop := benchmarkLogger(b, "info")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", M("request_id", i), M("ok", true))
	}

	b.StopTimer()
	stop()
	if dropped := log.DroppedCount(); dropped != 0 {
		b.Fatalf("benchmark dropped %d messages", dropped)
	}
}

func BenchmarkLoggerFilteredInfo(b *testing.B) {
	log, stop := benchmarkLogger(b, "error")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		log.Info("filtered benchmark message")
	}

	b.StopTimer()
	stop()
}

func BenchmarkLoggerParallel(b *testing.B) {
	log, stop := benchmarkLogger(b, "info")
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.Info("parallel benchmark message")
		}
	})

	b.StopTimer()
	stop()
	if dropped := log.DroppedCount(); dropped != 0 {
		b.Fatalf("benchmark dropped %d messages", dropped)
	}
}

func BenchmarkM(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = M("request_id", i)
	}
}
