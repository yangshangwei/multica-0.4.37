package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type iterationBenchmarkMetrics struct {
	queries atomic.Int64
	lockNS  atomic.Int64
}
type iterationBenchmarkStarter struct {
	txStarter
	metrics *iterationBenchmarkMetrics
}

func (s iterationBenchmarkStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationBenchmarkTx{Tx: tx, metrics: s.metrics}, nil
}

type iterationBenchmarkTx struct {
	pgx.Tx
	metrics *iterationBenchmarkMetrics
}

func (tx *iterationBenchmarkTx) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	tx.metrics.queries.Add(int64(batch.Len()))
	started := time.Now()
	locks := true
	for _, query := range batch.QueuedQueries {
		if !strings.Contains(query.SQL, "Lock") && !strings.Contains(query.SQL, "lock") {
			locks = false
		}
	}
	return &iterationBenchmarkBatch{BatchResults: tx.Tx.SendBatch(ctx, batch), metrics: tx.metrics, started: started, locks: locks}
}

type iterationBenchmarkBatch struct {
	pgx.BatchResults
	metrics *iterationBenchmarkMetrics
	started time.Time
	locks   bool
	once    sync.Once
}

func (r *iterationBenchmarkBatch) Close() error {
	err := r.BatchResults.Close()
	r.once.Do(func() {
		if r.locks {
			r.metrics.lockNS.Add(int64(time.Since(r.started)))
		}
	})
	return err
}

func (tx *iterationBenchmarkTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.metrics.queries.Add(1)
	started := time.Now()
	result, err := tx.Tx.Exec(ctx, sql, args...)
	if strings.Contains(sql, "lock") || strings.Contains(sql, "Lock") {
		tx.metrics.lockNS.Add(int64(time.Since(started)))
	}
	return result, err
}
func (tx *iterationBenchmarkTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.metrics.queries.Add(1)
	return tx.Tx.Query(ctx, sql, args...)
}
func (tx *iterationBenchmarkTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.metrics.queries.Add(1)
	started := time.Now()
	row := tx.Tx.QueryRow(ctx, sql, args...)
	return &iterationBenchmarkRow{Row: row, started: started, metrics: tx.metrics, lock: strings.Contains(sql, "Lock") || strings.Contains(sql, "lock")}
}

type iterationBenchmarkRow struct {
	pgx.Row
	started time.Time
	metrics *iterationBenchmarkMetrics
	lock    bool
}

func (r *iterationBenchmarkRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	if r.lock {
		r.metrics.lockNS.Add(int64(time.Since(r.started)))
	}
	return err
}

// BenchmarkIterationOrdinaryWrite measures the actual HTTP update path with
// disabled iterations and distinct issues in one workspace. Keep this fixture
// stable so the pre-recorder baseline and subsequent writers are comparable.
func BenchmarkIterationOrdinaryWrite(b *testing.B) {
	if testHandler == nil {
		b.Fatal("a reachable isolated PostgreSQL database is required")
	}
	for _, concurrency := range []int{1, 4} {
		b.Run(fmt.Sprintf("writers_%d", concurrency), func(b *testing.B) {
			metrics := &iterationBenchmarkMetrics{}
			h := *testHandler
			h.TxStarter = iterationBenchmarkStarter{txStarter: h.TxStarter, metrics: metrics}
			ids := make([]string, concurrency)
			for i := range ids {
				ids[i] = dbfx.Issue(b, "ordinary write benchmark")
			}
			latencies := make([]time.Duration, b.N)
			var next atomic.Int64
			var failures atomic.Int64
			var workers sync.WaitGroup
			b.ResetTimer()
			start := time.Now()
			for _, id := range ids {
				workers.Add(1)
				go func(id string) {
					defer workers.Done()
					for {
						i := int(next.Add(1) - 1)
						if i >= b.N {
							return
						}
						req := withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, map[string]any{"title": fmt.Sprintf("ordinary write %d", i)}), "id", id)
						w := httptest.NewRecorder()
						started := time.Now()
						h.UpdateIssue(w, req)
						latencies[i] = time.Since(started)
						if w.Code != http.StatusOK {
							failures.Add(1)
						}
					}
				}(id)
			}
			workers.Wait()
			elapsed := time.Since(start)
			b.StopTimer()
			if failures.Load() != 0 {
				b.Fatalf("%d HTTP updates failed", failures.Load())
			}
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			b.ReportMetric(float64(latencies[(len(latencies)-1)*95/100].Microseconds())/1000, "p95-ms")
			b.ReportMetric(float64(b.N)/elapsed.Seconds(), "writes/s")
			b.ReportMetric(float64(metrics.queries.Load())/float64(b.N), "tx-queries/write")
			b.ReportMetric(float64(metrics.lockNS.Load())/float64(b.N)/1e6, "lock-statement-ms/write")
		})
	}
}
