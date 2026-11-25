package service
import (
 "context"
 "errors"
 "reflect"
 "strings"
 "testing"
 "time"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgconn"
 "github.com/multica-ai/multica/server/internal/auth"
 "github.com/multica-ai/multica/server/pkg/protocol"
)
type acceptanceCapabilityTrace struct{ TxStarter; states []string; pids []uint32 }
type acceptanceCapabilityTx struct{ pgx.Tx; trace *acceptanceCapabilityTrace }
type acceptanceCapabilityRows struct { pgx.Rows; trace *acceptanceCapabilityTrace; pid uint32; observed bool }
func (s *acceptanceCapabilityTrace) Begin(ctx context.Context)(pgx.Tx,error){tx,err:=s.TxStarter.Begin(ctx);if err!=nil{return nil,err};return &acceptanceCapabilityTx{tx,s},nil}
func (s *acceptanceCapabilityTrace) record(err error,pid uint32){var pgerr *pgconn.PgError;if errors.As(err,&pgerr){s.states=append(s.states,pgerr.Code);s.pids=append(s.pids,pid)}}
func (tx *acceptanceCapabilityTx) Query(ctx context.Context, sql string,args ...any)(pgx.Rows,error){
 rows,err:=tx.Tx.Query(ctx,sql,args...)
 if !strings.Contains(sql,"-- name: LockArtifactUsers "){return rows,err}
 if err!=nil{tx.trace.record(err,tx.Conn().PgConn().PID());return rows,err}
 return &acceptanceCapabilityRows{Rows:rows,trace:tx.trace,pid:tx.Conn().PgConn().PID()},nil
}
func(r *acceptanceCapabilityRows)Err()error{err:=r.Rows.Err();if !r.observed{r.observed=true;r.trace.record(err,r.pid)};return err}
func TestAcceptanceCapabilityAccountContentionControl(t *testing.T){
 f:=newManagedCollaborationFixture(t,"local")
 registration,err:=f.svc.RegisterRuntimeCapabilities(f.daemonCtx,hostCapabilityRegistration(f));if err!=nil{t.Fatal(err)}
 f.runtime=mustRuntime(t,f.svc.Queries,f.runtime.ID)
 before,err:=f.svc.RuntimeCapabilities(f.daemonCtx,f.runtime.ID);if err!=nil||!reflect.DeepEqual(before,registration){t.Fatalf("baseline capability failed: %v",err)}
 session,ok:=auth.PasswordSessionFromContext(f.daemonCtx);if !ok||session.UserID==""{t.Fatal("missing real fixture password session")}
 holder,err:=f.pool.Begin(f.daemonCtx);if err!=nil{t.Fatal(err)}
 defer holder.Rollback(context.Background())
 if _,err=auth.LockPasswordSession(f.daemonCtx,f.svc.Queries.WithTx(holder));err!=nil{t.Fatal(err)}
 holderPID:=holder.Conn().PgConn().PID()
 reader:=*f.svc;trace:=&acceptanceCapabilityTrace{TxStarter:f.pool};reader.TxStarter=trace
 ctx,cancel:=context.WithTimeout(f.daemonCtx,5*time.Second);defer cancel()
 started:=time.Now();got,err:=reader.RuntimeCapabilities(ctx,f.runtime.ID)
 if !collaborationErrorCode(err,"collaboration_busy")||!reflect.DeepEqual(got,protocol.CollaborationRuntimeCapabilitiesResponse{})||ctx.Err()!=nil{t.Fatalf("held writer response=%+v error=%v context=%v",got,err,ctx.Err())}
 if len(trace.states)!=3{t.Fatalf("conflicts=%v want three",trace.states)}
 for i,code:=range trace.states{if code!="55P03"||trace.pids[i]==holderPID{t.Fatalf("unexpected conflict state=%s holder=%d reader=%d",code,holderPID,trace.pids[i])}}
 t.Logf("held actual LockPasswordSession: holder_pid=%d reader_pids=%v query=LockArtifactUsers sqlstates=%v elapsed=%s typed_response=503/collaboration_busy context=none",holderPID,trace.pids,trace.states,time.Since(started))
 if err=holder.Rollback(f.daemonCtx);err!=nil{t.Fatal(err)}
 recovered,err:=reader.RuntimeCapabilities(ctx,f.runtime.ID);if err!=nil||!reflect.DeepEqual(recovered,before){t.Fatalf("released writer did not recover exact capability: %v",err)}
 t.Log("released actual account writer: exact capability recovered; no policy, retry or budget changed")
}
