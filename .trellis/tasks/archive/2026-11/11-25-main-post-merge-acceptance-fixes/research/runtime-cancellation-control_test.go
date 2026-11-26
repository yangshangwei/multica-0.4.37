package middleware

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "log"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
)

func TestAcceptanceCancellationLogControl(t *testing.T) {
 previous := slog.Default()
 writer, flags := log.Writer(), log.Flags()
 defer func(){ slog.SetDefault(previous); log.SetOutput(writer); log.SetFlags(flags) }()
 var logs bytes.Buffer
 slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
 entered, finished := make(chan struct{}), make(chan struct{})
 server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   close(entered)
   <-r.Context().Done()
   if !errors.Is(r.Context().Err(), context.Canceled) { t.Error("server context is not canceled") }
   passwordAuthError(w, r.Context().Err())
  })).ServeHTTP(w, r)
  close(finished)
 }))
 defer server.Close()
 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()
 request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/collaboration/issues/control/knowledge-candidates", nil)
 if err != nil {t.Fatal(err)}
 result := make(chan error,1)
 go func(){ response, err := server.Client().Do(request); if response != nil { response.Body.Close() }; result <- err }()
 select {case <-entered: case <-time.After(2*time.Second): t.Fatal("server not entered")}
 cancel()
 select {case err := <-result: if !errors.Is(err, context.Canceled) {t.Fatalf("client error = %v",err)}; case <-time.After(2*time.Second): t.Fatal("client not canceled")}
 select {case <-finished: case <-time.After(2*time.Second):t.Fatal("server not finished")}
 var captured map[string]any
 if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &captured); err != nil {t.Fatal(err)}
 if captured["status"] != float64(503) || captured["level"] != "ERROR" {t.Fatalf("log = %v",captured)}
 if _, ok := captured["request_context"]; ok {t.Fatal("baseline unexpectedly differentiates request context")}
 t.Logf("actual RequestLogger after client cancellation: %s", strings.TrimSpace(logs.String()))
 logs.Reset()
 healthyRequest := httptest.NewRequest(http.MethodGet,"/api/collaboration/issues/control/knowledge-candidates", nil)
 response := httptest.NewRecorder()
 RequestLogger(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){passwordAuthError(w, errors.New("synthetic database failure"))})).ServeHTTP(response,healthyRequest)
 if response.Code != 503 || !strings.Contains(response.Body.String(), "auth_unavailable") || healthyRequest.Context().Err()!=nil {t.Fatal("noncancelled unavailable control failed")}
 t.Logf("actual RequestLogger for uncancelled failure: %s", strings.TrimSpace(logs.String()))
}
