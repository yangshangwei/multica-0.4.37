package service

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func pickerSuffix() string { return "pk" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12] }

func pickerAccount(t *testing.T, f *collaborationFixture, name string, over ...testutil.Cols) string {
	t.Helper()
	return f.fx.User(t, name, "picker-"+uuid.NewString()+"@example.test", over...)
}

func pickerIDs(items []CollaborationRecipientCandidate) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, util.UUIDToString(item.UserID))
	}
	return out
}

// pickerPages walks every page and fails on a short page before the last one.
func pickerPages(t *testing.T, f *collaborationQueueFixture, query CollaborationRecipientSearchQuery) []string {
	t.Helper()
	var out []string
	for page := 0; ; page++ {
		result, err := f.svc.SearchRecipients(f.ctx, query)
		if err != nil || page > 100 {
			t.Fatalf("page %d: %v", page, err)
		}
		out = append(out, pickerIDs(result.Recipients)...)
		if result.NextCursor == "" {
			return out
		}
		if query.Limit != nil && len(result.Recipients) != *query.Limit {
			t.Fatalf("page %d has %d rows before the last page", page, len(result.Recipients))
		}
		query.Cursor = result.NextCursor
	}
}

func TestCollaborationRecipientSearchPagingIsStableAcrossSharedNames(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	suffix := pickerSuffix()
	var accounts []string
	for range 26 {
		accounts = append(accounts, pickerAccount(t, f.collaborationFixture, "Same Name "+suffix))
	}
	for _, name := range []string{"Aaron " + suffix, "zed " + suffix, "SAME NAME " + strings.ToUpper(suffix), "张三 " + suffix, "Same Name " + suffix + " Jr"} {
		accounts = append(accounts, pickerAccount(t, f.collaborationFixture, name))
	}
	var want []string
	rows, err := f.pool.Query(f.ctx, `SELECT id::text FROM "user" WHERE id = ANY($1::uuid[]) ORDER BY lower(name), id`, accounts)
	if err == nil {
		want, err = pgx.CollectRows(rows, pgx.RowTo[string])
	}
	if err != nil || len(want) != 31 {
		t.Fatalf("expected order: %v, %v", want, err)
	}
	for _, limit := range []int{1, 4, 31, 50} {
		got := pickerPages(t, f, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: strings.ToUpper(suffix), Limit: &limit, Directory: true})
		if !slices.Equal(got, want) {
			t.Fatalf("limit %d paged %d rows, want the %d rows in (lower(name), id) order without duplicates or gaps:\n%v\n%v", limit, len(got), len(want), got, want)
		}
	}
	first, err := f.svc.SearchRecipients(f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: suffix, Directory: true})
	if err != nil || len(first.Recipients) != 30 || first.NextCursor == "" || len(first.Recent) != 0 {
		t.Fatalf("default page: %d rows, cursor %q, %d recent, %v", len(first.Recipients), first.NextCursor, len(first.Recent), err)
	}
}

func TestCollaborationRecipientSearchExcludesUnavailableAccountsBeforePaging(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	suffix := pickerSuffix()
	name := "Probe " + suffix
	visible := pickerAccount(t, f.collaborationFixture, name)
	noEmail := pickerAccount(t, f.collaborationFixture, name)
	f.fx.Exec(t, `UPDATE "user" SET email=NULL WHERE id=$1`, noEmail)
	disabled := pickerAccount(t, f.collaborationFixture, name)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, disabled)
	restricted := pickerAccount(t, f.collaborationFixture, name)
	recipientCredential(t, f.collaborationFixture, restricted, "restricted_"+suffix, true)
	deniedEmail := pickerAccount(t, f.collaborationFixture, name)
	// A case/whitespace variant of the denylisted mailbox, as stored historically.
	f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, deniedEmail, "\t GTWTROX@Mowan666.com\r\n")
	deniedID := pickerAccount(t, f.collaborationFixture, name, testutil.Cols{"id": "1d542296-17c6-484a-9914-dcee589be116"})
	f.fx.Exec(t, `UPDATE "user" SET name=$2 WHERE id=$1`, f.creator, name)
	one := 1
	query := CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: suffix, Limit: &one, Directory: true}
	want := []string{visible, noEmail}
	slices.Sort(want)
	if got := pickerPages(t, f, query); !slices.Equal(got, want) {
		t.Fatalf("listed %v, want only %v (disabled %s, restricted %s, denied %s/%s, caller %s)", got, want, disabled, restricted, deniedEmail, deniedID, f.creator)
	}

	t.Setenv("MULTICA_AUTH_MODE", "password")
	recipientCredential(t, f.collaborationFixture, f.creator, "creator_"+suffix, false)
	recipientCredential(t, f.collaborationFixture, visible, "visible_"+suffix, false)
	f.ctx = auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.creator, Version: 1, Kind: "jwt"})
	if got := pickerPages(t, f, query); !slices.Equal(got, []string{visible}) {
		t.Fatalf("password mode listed %v; an account without a credential must be hidden", got)
	}
}

func TestCollaborationRecipientSearchMatchesLiteralNameSubstringsOnly(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	suffix := pickerSuffix()
	percent := pickerAccount(t, f.collaborationFixture, "100%"+suffix)
	underscore := pickerAccount(t, f.collaborationFixture, "a_"+suffix)
	backslash := pickerAccount(t, f.collaborationFixture, `x\`+suffix)
	plain := pickerAccount(t, f.collaborationFixture, "plain xx"+suffix)
	mailbox := "mailbox" + pickerSuffix() + "@example.test"
	username := "user_" + pickerSuffix()
	f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, plain, mailbox)
	recipientCredential(t, f.collaborationFixture, plain, username, false)
	search := func(q string) []string {
		t.Helper()
		result, err := f.svc.SearchRecipients(f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: q, Directory: true})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		got := pickerIDs(result.Recipients)
		slices.Sort(got)
		return got
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }
	for _, scenario := range []struct {
		q    string
		want []string
	}{
		{suffix, sorted(percent, underscore, backslash, plain)},
		{"%" + suffix, sorted(percent)},
		{"_" + suffix, sorted(underscore)},
		{`\` + suffix, sorted(backslash)},
		{"\t " + strings.ToUpper("PLAIN XX"+suffix) + "\u3000", sorted(plain)},
		{mailbox, nil},
		{strings.TrimSuffix(mailbox, "@example.test"), nil},
		{username, nil},
		{plain, nil},
		{strings.ReplaceAll(plain, "-", ""), nil},
	} {
		if got := search(scenario.q); !slices.Equal(got, scenario.want) {
			t.Fatalf("q %q matched %v, want %v", scenario.q, got, scenario.want)
		}
	}
}

func TestCollaborationRecipientSearchRejectsInvalidInputBeforeDatabase(t *testing.T) {
	svc := NewCollaborationService(nil, nil, nil)
	ctx := collaborationContext(uuid.NewString())
	source := util.MustParseUUID(uuid.NewString())
	cursor := func(name, id string) string {
		value, err := encodeCollaborationRecipientCursor(name, util.MustParseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	valid := cursor("name", uuid.NewString())
	limit := func(n int) *int { return &n }
	for name, query := range map[string]CollaborationRecipientSearchQuery{
		"long_query":       {Query: strings.Repeat("名", 65)},
		"nul_query":        {Query: "a\x00b"},
		"invalid_utf8":     {Query: "a\xffb"},
		"zero_limit":       {Limit: limit(0)},
		"negative_limit":   {Limit: limit(-1)},
		"large_limit":      {Limit: limit(51)},
		"not_base64":       {Cursor: "%%%"},
		"padded_base64":    {Cursor: valid + "="},
		"not_json":         {Cursor: "bm90LWpzb24"},
		"missing_id":       {Cursor: "eyJuIjoieCJ9"},
		"missing_name":     {Cursor: "eyJpIjoiMDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAxIn0"},
		"bad_id":           {Cursor: "eyJuIjoieCIsImkiOiJNVUwtMSJ9"},
		"unknown_key":      {Cursor: "eyJuIjoieCIsImkiOiIwMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDEiLCJ4IjoxfQ"},
		"trailing_json":    {Cursor: "eyJuIjoieCIsImkiOiIwMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDEifXt9"},
		"long_cursor_name": {Cursor: cursor(strings.Repeat("n", 1025), uuid.NewString())},
		"nul_cursor_name":  {Cursor: cursor("a\x00b", uuid.NewString())},
	} {
		t.Run(name, func(t *testing.T) {
			query.SourceWorkspaceID = source
			_, err := svc.SearchRecipients(ctx, query)
			wantRecipientError(t, err, http.StatusBadRequest, "invalid_recipient_query")
		})
	}
	// Boundary values pass validation and only then meet the missing database.
	for _, query := range []CollaborationRecipientSearchQuery{
		{Query: strings.Repeat("名", 64)}, {Limit: limit(1)}, {Limit: limit(50)}, {Cursor: cursor(strings.Repeat("n", 1024), uuid.NewString())}, {Cursor: valid},
	} {
		query.SourceWorkspaceID = source
		_, err := svc.SearchRecipients(ctx, query)
		wantRecipientError(t, err, http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	for _, query := range []CollaborationRecipientSearchQuery{{}, {IssueID: source, SourceWorkspaceID: source}} {
		_, err := svc.SearchRecipients(ctx, query)
		wantRecipientError(t, err, http.StatusBadRequest, "invalid_recipient_context")
	}
	_, err := svc.SearchRecipients(auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: uuid.NewString(), Kind: auth.CredentialTask}), CollaborationRecipientSearchQuery{SourceWorkspaceID: source})
	wantRecipientError(t, err, http.StatusForbidden, "forbidden")
}
