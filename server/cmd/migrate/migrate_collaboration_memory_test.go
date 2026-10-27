package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// 743 alters collaboration_run, which the private rehearsal schema does not
// contain; its additive ALTER is exercised by the full migrate-up chain.
var collaborationMemoryMigrationVersions = []string{
	"739_collaboration_memory_version", "740_collaboration_memory_version_id",
	"741_collaboration_memory_version_issue", "742_collaboration_memory_version_run_request",
}

func collaborationMemoryMigrationState(t *testing.T, fx *testutil.Fixture, schema string) string {
	t.Helper()
	var rows string
	if err := fx.Pool.QueryRow(t.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(v) ORDER BY id),'[]'::jsonb)::text FROM collaboration_memory_version v`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return iterationRecoveryCatalog(t, t.Context(), fx.Pool, schema) + rows
}

func memberMemoryCols(issue string, version int, content string) testutil.Cols {
	return testutil.Cols{"issue_id": issue, "version": version, "content": content, "bytes": len(content),
		"author_kind": "member", "author_user_id": uuid.NewString()}
}

func TestCollaborationMemoryMigrationsPreserveHistoryAndRefuseDowngrade(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("populated_%t", populated), func(t *testing.T) {
			ctx, fx, schema, opts := switchMigrationFixture(t)
			opts.Files = realMigrationFiles(t, collaborationMemoryMigrationVersions, "up")
			first := opts
			first.Files = first.Files[:1]
			if err := runMigrations(ctx, fx.Pool, first); err != nil {
				t.Fatal(err)
			}
			if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_memory_version'::regclass`) != 0 {
				t.Fatal("version table creation introduced an inline index")
			}
			if populated {
				fx.Insert(t, "collaboration_memory_version", memberMemoryCols(uuid.NewString(), 1, "retained"))
			}
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_memory_version'::regclass AND indisunique AND indisvalid AND indisready AND indislive`) != 3 ||
				fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='collaboration_memory_version'::regclass AND contype IN ('f','p')`) != 0 {
				t.Fatal("versions lack three usable concurrent unique indexes or gained an FK/inline primary key")
			}
			before := collaborationMemoryMigrationState(t, fx, schema)
			if err := runMigrations(ctx, fx.Pool, opts); err != nil {
				t.Fatal(err)
			}
			if after := collaborationMemoryMigrationState(t, fx, schema); after != before {
				t.Fatal("repeated up changed version history, ledger timestamps or index identity")
			}
			reverse := slices.Clone(collaborationMemoryMigrationVersions)
			slices.Reverse(reverse)
			for _, version := range reverse {
				down := opts
				down.Direction, down.Files, down.Hooks = "down", realMigrationFiles(t, []string{version}, "down"), hooksForDirection("down")
				var refusal *pgconn.PgError
				if err := runMigrations(ctx, fx.Pool, down); !errors.As(err, &refusal) || refusal.Code != "P0001" {
					t.Fatalf("%s downgrade did not refuse, including empty maintenance state: %v", version, err)
				}
				if after := collaborationMemoryMigrationState(t, fx, schema); after != before {
					t.Fatal("refused down changed catalog, ledger or version history")
				}
			}
		})
	}
}

func TestCollaborationMemoryMigrationsEnforceVersionConstraints(t *testing.T) {
	ctx, fx, _, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, collaborationMemoryMigrationVersions, "up")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	issue := uuid.NewString()
	id := fx.Insert(t, "collaboration_memory_version", memberMemoryCols(issue, 1, "first"))
	fx.Insert(t, "collaboration_memory_version", memberMemoryCols(issue, 2, "second"))
	agent := testutil.Cols{"issue_id": issue, "version": 3, "content": "ai", "bytes": 2, "author_kind": "agent",
		"agent_id": uuid.NewString(), "run_id": uuid.NewString(), "resource_owner_user_id": uuid.NewString(), "request_id": uuid.NewString(),
		"request_hash": strings.Repeat("a", 64)}
	agentID := fx.Insert(t, "collaboration_memory_version", agent)
	refuse := func(sql string, args ...any) {
		t.Helper()
		var constraint *pgconn.PgError
		if _, err := fx.Pool.Exec(ctx, sql, args...); !errors.As(err, &constraint) || constraint.Code != "23514" && constraint.Code != "23505" {
			t.Fatalf("%s was not rejected by a constraint: %v", sql, err)
		}
	}
	insert := `INSERT INTO collaboration_memory_version (issue_id, version, content, bytes, author_kind, author_user_id, agent_id, run_id, resource_owner_user_id, request_id, restored_from_version, request_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	member, other := uuid.NewString(), uuid.NewString()
	for name, args := range map[string][]any{
		"version zero":            {issue, 0, "x", 1, "member", member, nil, nil, nil, nil, nil},
		"duplicate version":       {issue, 2, "x", 1, "member", member, nil, nil, nil, nil, nil},
		"bytes mismatch":          {issue, 4, "界", 1, "member", member, nil, nil, nil, nil, nil},
		"hard ceiling":            {issue, 4, strings.Repeat("a", 524289), 524289, "member", member, nil, nil, nil, nil, nil},
		"unknown author":          {issue, 4, "x", 1, "system", member, nil, nil, nil, nil, nil},
		"member with run":         {issue, 4, "x", 1, "member", member, nil, other, nil, nil, nil},
		"agent without run":       {issue, 4, "x", 1, "agent", nil, other, nil, other, "r", nil},
		"agent restore":           {issue, 4, "x", 1, "agent", nil, other, other, other, "r", 1},
		"future restore source":   {issue, 4, "x", 1, "member", member, nil, nil, nil, nil, 4},
		"duplicate run request":   {issue, 4, "x", 1, "agent", nil, other, agent["run_id"], other, agent["request_id"], nil},
		"overlong request id":     {issue, 4, "x", 1, "agent", nil, other, other, other, strings.Repeat("r", 129), nil},
		"member with request key": {issue, 4, "x", 1, "member", member, nil, nil, nil, "r", nil},
		"agent without hash":      {issue, 4, "x", 1, "agent", nil, other, other, other, "r", nil, nil},
		"malformed request hash":  {issue, 4, "x", 1, "agent", nil, other, other, other, "r", nil, "not-a-hash"},
		"member with hash":        {issue, 4, "x", 1, "member", member, nil, nil, nil, nil, nil, strings.Repeat("a", 64)},
	} {
		if len(args) == 11 {
			var hash any
			if args[4] == "agent" {
				hash = strings.Repeat("a", 64)
			}
			args = append(args, hash)
		}
		t.Run(name, func(t *testing.T) { refuse(insert, args...) })
	}
	// A member row needs no run identity; the partial index does not apply.
	fx.Insert(t, "collaboration_memory_version", memberMemoryCols(uuid.NewString(), 1, "other issue"))
	// Versions are immutable except for one purge that clears the text.
	refuse(`UPDATE collaboration_memory_version SET content='rewritten', bytes=9 WHERE id=$1`, id)
	refuse(`UPDATE collaboration_memory_version SET version=9 WHERE id=$1`, id)
	refuse(`UPDATE collaboration_memory_version SET author_user_id=gen_random_uuid() WHERE id=$1`, id)
	refuse(`UPDATE collaboration_memory_version SET purged_at=now(), purged_by_user_id=gen_random_uuid(), purge_reason='r' WHERE id=$1`, id)
	refuse(`UPDATE collaboration_memory_version SET purged_by_admin=true WHERE id=$1`, agentID)
	refuse(`UPDATE collaboration_memory_version SET request_hash=$2 WHERE id=$1`, agentID, strings.Repeat("b", 64))
	fx.Exec(t, `UPDATE collaboration_memory_version SET content='', purged_at=now(), purged_by_user_id=gen_random_uuid(), purge_reason='credential' WHERE id=$1`, id)
	if fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE id=$1 AND content='' AND bytes=5 AND purged_at IS NOT NULL`, id) != 1 {
		t.Fatal("purge did not retain the original byte count")
	}
	refuse(`UPDATE collaboration_memory_version SET purge_reason='changed' WHERE id=$1`, id)
	refuse(`UPDATE collaboration_memory_version SET purged_at=NULL, purged_by_user_id=NULL, purge_reason=NULL WHERE id=$1`, id)
	fx.Exec(t, `UPDATE collaboration_memory_version SET content='', purged_at=now(), purged_by_user_id=gen_random_uuid(), purge_reason='credential' WHERE id=$1`, agentID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE id=$1 AND content='' AND request_hash=$2 AND purged_at IS NOT NULL`, agentID, strings.Repeat("a", 64)) != 1 {
		t.Fatal("purge discarded the original request fingerprint")
	}
}
