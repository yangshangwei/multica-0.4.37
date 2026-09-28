package migrations

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestBuiltinAgentAvatarMigrationPreservesCustomizations(t *testing.T) {
	ctx := context.Background()
	conn, _ := setupBuiltinAgentDisplayNameMigration(t)
	if _, err := conn.Exec(ctx, `ALTER TABLE agent ADD COLUMN avatar_url TEXT`); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(conn.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig = conn.Config().Copy()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fixture := testutil.New(pool, "", "")
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		label     string
		systemKey any
		avatar    any
		changed   bool
	}{
		{"built-in default", "mika", "emoji:🦄", true},
		{"custom image", "mika", "https://example.test/custom.png", false},
		{"custom emoji", "mika", "emoji:🦉", false},
		{"cleared", "mika", nil, false},
		{"empty", "mika", "", false},
		{"ordinary unicorn", nil, "emoji:🦄", false},
		{"ordinary seal", nil, "/api/avatars/builtin/afu-seal-v1.png", false},
		{"another system agent", "builder", "emoji:🦄", false},
		{"already current", "mika", "/api/avatars/builtin/afu-seal-v1.png", false},
	}
	type expected struct {
		id, preserved string
	}
	rows := make([]expected, len(cases))
	for i, tc := range cases {
		rows[i].id = fixture.Insert(t, "agent", testutil.Cols{
			"id": uuid.NewString(), "workspace_id": uuid.NewString(),
			"name": tc.label, "system_key": tc.systemKey, "avatar_url": tc.avatar,
			"instructions": "Keep owner instructions", "updated_at": before,
		})
		if err := conn.QueryRow(ctx, `SELECT (to_jsonb(agent) - 'avatar_url' - 'updated_at')::text FROM agent WHERE id = $1`, rows[i].id).Scan(&rows[i].preserved); err != nil {
			t.Fatal(err)
		}
	}

	applyMigrationFile(t, ctx, conn, "459_builtin_agent_avatar.up.sql")
	for i, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			var avatar pgtype.Text
			var updatedAt time.Time
			var preserved string
			if err := conn.QueryRow(ctx, `SELECT avatar_url, updated_at, (to_jsonb(agent) - 'avatar_url' - 'updated_at')::text FROM agent WHERE id = $1`, rows[i].id).Scan(&avatar, &updatedAt, &preserved); err != nil {
				t.Fatal(err)
			}
			want := tc.avatar
			if tc.changed {
				want = "/api/avatars/builtin/afu-seal-v1.png"
			}
			if want == nil && avatar.Valid || want != nil && (!avatar.Valid || avatar.String != want) {
				t.Errorf("avatar = %+v, want %v", avatar, want)
			}
			if preserved != rows[i].preserved {
				t.Error("migration changed identity, instructions or other metadata")
			}
			if tc.changed && !updatedAt.After(before) || !tc.changed && !updatedAt.Equal(before) {
				t.Errorf("updated_at = %v, changed = %t, original = %v", updatedAt, tc.changed, before)
			}
		})
	}
	after := snapshotBuiltinAgentMigrationTables(t, conn, "agent")
	applyMigrationFile(t, ctx, conn, "459_builtin_agent_avatar.up.sql")
	if got := snapshotBuiltinAgentMigrationTables(t, conn, "agent"); !reflect.DeepEqual(got, after) {
		t.Error("replaying migration changed avatars or timestamps")
	}

	fixture.Exec(t, `UPDATE agent SET avatar_url = 'https://example.test/after.png' WHERE id = $1`, rows[0].id)
	beforeDown := snapshotBuiltinAgentMigrationTables(t, conn, "agent")
	applyMigrationFile(t, ctx, conn, "459_builtin_agent_avatar.down.sql")
	var reverted pgtype.Text
	var revertedAt time.Time
	last := rows[len(rows)-1]
	if err := conn.QueryRow(ctx, `SELECT avatar_url, updated_at FROM agent WHERE id = $1`, last.id).Scan(&reverted, &revertedAt); err != nil {
		t.Fatal(err)
	}
	if !reverted.Valid || reverted.String != "emoji:🦄" || !revertedAt.After(before) {
		t.Errorf("rollback failed to restore a usable default: avatar=%+v updated_at=%v", reverted, revertedAt)
	}
	afterDown := snapshotBuiltinAgentMigrationTables(t, conn, "agent")
	applyMigrationFile(t, ctx, conn, "459_builtin_agent_avatar.down.sql")
	if got := snapshotBuiltinAgentMigrationTables(t, conn, "agent"); !reflect.DeepEqual(got, afterDown) {
		t.Error("replaying rollback changed avatars or timestamps")
	}
	// Restore only the expected rollback fields to compare all other rows and
	// metadata, including the custom avatar chosen after the up migration.
	fixture.Exec(t, `UPDATE agent SET avatar_url = '/api/avatars/builtin/afu-seal-v1.png', updated_at = $2 WHERE id = $1`, last.id, before)
	if got := snapshotBuiltinAgentMigrationTables(t, conn, "agent"); !reflect.DeepEqual(got, beforeDown) {
		t.Error("rollback changed a custom, cleared or ordinary avatar, or unrelated metadata")
	}
}
