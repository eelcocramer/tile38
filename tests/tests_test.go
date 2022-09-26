package tests

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	clear   = "\x1b[0m"
	bright  = "\x1b[1m"
	dim     = "\x1b[2m"
	black   = "\x1b[30m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
	white   = "\x1b[37m"
)

func TestAll(t *testing.T) {

	mockCleanup(false)
	defer mockCleanup(false)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		mockCleanup(false)
		os.Exit(1)
	}()

	runSubTest(t, "keys", subTestKeys)
	runSubTest(t, "json", subTestJSON)
	runSubTest(t, "search", subTestSearch)
	runSubTest(t, "testcmd", subTestTestCmd)
	runSubTest(t, "client", subTestClient)
	runSubTest(t, "scripts", subTestScripts)
	runSubTest(t, "fence", subTestFence)
	runSubTest(t, "info", subTestInfo)
	runSubTest(t, "timeouts", subTestTimeout)
	runSubTest(t, "metrics", subTestMetrics)
	runSubTest(t, "aof", subTestAOF)
}

func runSubTest(t *testing.T, name string, test func(t *testing.T, mc *mockServer)) {
	t.Run(name, func(t *testing.T) {
		// t.Parallel()
		t.Helper()

		mc, err := mockOpenServer(MockServerOptions{
			Silent:  true,
			Metrics: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer mc.Close()

		fmt.Printf(bright+"Testing %s\n"+clear, name)
		test(t, mc)
	})
}

func runStep(t *testing.T, mc *mockServer, name string, step func(mc *mockServer) error) {
	t.Run(name, func(t *testing.T) {
		t.Helper()
		if err := func() error {
			// reset the current server
			mc.ResetConn()
			defer mc.ResetConn()
			// clear the database so the test is consistent
			if err := mc.DoBatch(
				Do("OUTPUT", "resp").OK(),
				Do("FLUSHDB").OK(),
			); err != nil {
				return err
			}
			if err := step(mc); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			fmt.Fprintf(os.Stderr, "["+red+"fail"+clear+"]: %s\n", name)
			t.Fatal(err)
			// t.Fatal(err)
		}
		fmt.Printf("["+green+"ok"+clear+"]: %s\n", name)
	})
}

func BenchmarkAll(b *testing.B) {
	mockCleanup(true)
	defer mockCleanup(true)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		mockCleanup(true)
		os.Exit(1)
	}()

	mc, err := mockOpenServer(MockServerOptions{
		Silent: true, Metrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer mc.Close()
	runSubBenchmark(b, "search", mc, subBenchSearch)
}

func loadBenchmarkPoints(b *testing.B, mc *mockServer) (err error) {
	const nPoints = 200000
	rand.Seed(time.Now().UnixNano())

	// add a bunch of points
	for i := 0; i < nPoints; i++ {
		val := fmt.Sprintf("val:%d", i)
		var resp string
		var lat, lon, fval float64
		fval = rand.Float64()
		lat = rand.Float64()*180 - 90
		lon = rand.Float64()*360 - 180
		resp, err = redis.String(mc.conn.Do("SET",
			"mykey", val,
			"FIELD", "foo", fval,
			"POINT", lat, lon))
		if err != nil {
			return
		}
		if resp != "OK" {
			err = fmt.Errorf("expected 'OK', got '%s'", resp)
			return
		}
	}
	return
}

func runSubBenchmark(b *testing.B, name string, mc *mockServer, bench func(t *testing.B, mc *mockServer)) {
	b.Run(name, func(b *testing.B) {
		bench(b, mc)
	})
}

func runBenchStep(b *testing.B, mc *mockServer, name string, step func(mc *mockServer) error) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		b.Helper()
		if err := func() error {
			// reset the current server
			mc.ResetConn()
			defer mc.ResetConn()
			// clear the database so the test is consistent
			if err := mc.DoBatch([][]interface{}{
				{"OUTPUT", "resp"}, {"OK"},
				{"FLUSHDB"}, {"OK"},
			}); err != nil {
				return err
			}
			err := loadBenchmarkPoints(b, mc)
			if err != nil {
				return err
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := step(mc); err != nil {
					return err
				}
			}
			return nil
		}(); err != nil {
			b.Fatal(err)
		}
	})
}
