package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestPasswordRevocationDisconnectsTargetNotActingAdmin(t *testing.T) {
	passwordTestSetup(t)
	acting := passwordRegister(t, fmt.Sprintf("acting%d", time.Now().UnixNano()))
	target := passwordRegister(t, fmt.Sprintf("target%d", time.Now().UnixNano()))
	hub := realtime.NewHub()
	hub.SetPasswordQueries(testHandler.Queries)
	go hub.Run()
	daemonHub := daemonws.NewHub()
	daemonHub.SetPasswordQueries(testHandler.Queries)
	h := *testHandler
	h.Hub = hub
	h.DaemonHub = daemonHub
	h.TaskService = nil
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/daemon" {
			daemonHub.HandleWebSocket(w, r, daemonws.ClientIdentity{UserID: r.URL.Query().Get("user"), RuntimeIDs: []string{r.URL.Query().Get("user")}})
			return
		}
		realtime.HandleWebSocket(hub, passwordWSMember{}, nil, nil, w, r)
	}))
	defer srv.Close()
	connections := make(map[string][]*websocket.Conn)
	for _, user := range []LoginResponse{acting, target} {
		for _, daemon := range []bool{false, true} {
			path := "/?workspace_id=" + user.User.ID
			if daemon {
				path = "/daemon?user=" + user.User.ID
			}
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, http.Header{"Cookie": {auth.AuthCookieName + "=" + user.Token}, "Authorization": {"Bearer " + user.Token}})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			connections[user.User.ID] = append(connections[user.User.ID], c)
		}
		passwordWSWait(t, func() bool {
			return hub.HasLocalSubscribers(realtime.ScopeWorkspace, user.User.ID) && daemonHub.UserConnectionCount(user.User.ID) == 1
		})
	}
	req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/admin/users/"+target.User.ID+"/disable", nil), "X-User-ID", acting.User.ID)
	h.publishPasswordRevocation(req, target.User.ID, auth.PasswordRevocation{})
	for _, c := range connections[target.User.ID] {
		passwordWSClosed(t, c)
	}
	hub.BroadcastToWorkspace(acting.User.ID, []byte(`{"type":"actor-still-connected"}`))
	passwordWSRead(t, connections[acting.User.ID][0])
	daemonHub.NotifyTaskAvailable(acting.User.ID, "actor-task")
	passwordWSRead(t, connections[acting.User.ID][1])
}
