package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationAttachmentDeliveryMigrationsPreserveOriginalUploads(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_attachment_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	fx.Exec(t, "CREATE TABLE comment (id uuid, revision bigint NOT NULL DEFAULT 1)")
	fx.Exec(t, read("571_collaboration_schema.up"))
	versions := []string{"727_collaboration_attachment_delivery", "728_collaboration_attachment_upload_request"}
	for _, version := range versions {
		fx.Exec(t, read(version+".up"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
		WHERE c.relnamespace=current_schema()::regnamespace AND c.relname='collaboration_attachment_upload_request_idx' AND i.indisunique AND i.indisvalid`) != 1 {
		t.Fatal("upgrade did not create a usable original-upload identity index")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`) != 0 {
		t.Fatal("attachment upgrade added foreign keys")
	}
	id := fx.Insert(t, "collaboration_attachment", testutil.Cols{
		"draft_id": uuid.NewString(), "uploader_user_id": uuid.NewString(), "source_workspace_id": uuid.NewString(),
		"upload_request_id": uuid.NewString(), "content_sha256": strings.Repeat("a", 64), "encryption_key": make([]byte, 32),
		"storage_key": "private/collaboration/" + uuid.NewString(), "name": "retained.txt", "content_type": "text/plain", "size_bytes": 7,
	})
	_, err = pool.Exec(ctx, `INSERT INTO collaboration_attachment
		(draft_id,uploader_user_id,source_workspace_id,upload_request_id,content_sha256,encryption_key,storage_key,name,content_type,size_bytes)
		SELECT draft_id,uploader_user_id,source_workspace_id,upload_request_id,content_sha256,encryption_key,storage_key,name,content_type,size_bytes
		FROM collaboration_attachment WHERE id=$1`, id)
	var duplicate *pgconn.PgError
	if !errors.As(err, &duplicate) || duplicate.Code != "23505" {
		t.Fatalf("duplicate original upload was not refused: %v", err)
	}
	fx.Exec(t, `UPDATE collaboration_attachment SET issue_id=$2, delivered_at=now() WHERE id=$1`, id, uuid.NewString())
	var before string
	fx.QueryRow(t, `SELECT to_jsonb(a)::text FROM collaboration_attachment a WHERE id=$1`, id).Scan(&before)
	for _, version := range versions {
		if _, err = pool.Exec(ctx, read(version+".down")); err == nil || !strings.Contains(err.Error(), "retained collaboration attachment") {
			t.Fatalf("%s removed retained delivery or identity: %v", version, err)
		}
	}
	var after string
	fx.QueryRow(t, `SELECT to_jsonb(a)::text FROM collaboration_attachment a WHERE id=$1`, id).Scan(&after)
	if before != after {
		t.Fatal("refused rollback changed immutable attachment history")
	}
	fx.Exec(t, `DELETE FROM collaboration_attachment WHERE id=$1`, id)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `INSERT INTO collaboration_attachment
		(draft_id,uploader_user_id,storage_key,name,content_type,size_bytes)
		VALUES ($1,$2,'private/collaboration/pending','pending.txt','text/plain',0)`, uuid.NewString(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err = pool.Exec(bounded, read(version+".down"))
		cancel()
		var busy *pgconn.PgError
		if !errors.As(err, &busy) || busy.Code != "55P03" {
			t.Fatalf("%s did not refuse an uncommitted first writer: %v", version, err)
		}
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for i := len(versions) - 1; i >= 0; i-- {
		fx.Exec(t, read(versions[i]+".down"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname='collaboration_attachment_upload_request_idx'`) != 0 ||
		fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='collaboration_attachment' AND column_name='encryption_key'`) != 0 {
		t.Fatal("empty maintenance rollback left the new attachment schema behind")
	}
}
