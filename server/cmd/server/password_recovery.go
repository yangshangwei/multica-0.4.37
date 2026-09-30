package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// runPasswordRecovery is dispatched before listeners or background jobs start.
// The password is read from stdin, never a flag, environment variable or log.
func runPasswordRecovery(args []string) error {
	if !auth.PasswordMode() {
		return errors.New("password recovery requires MULTICA_AUTH_MODE=password")
	}
	flags := flag.NewFlagSet("password-recover", flag.ContinueOnError)
	userID := flags.String("user", "", "Existing user UUID")
	username := flags.String("username", "", "Username for an account without password credentials")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	id, err := util.ParseUUID(*userID)
	if err != nil {
		return errors.New("--user must be an existing user UUID")
	}
	secret, err := readRecoveryPassword()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hash, err := auth.HashPassword(ctx, secret)
	secret = ""
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if _, err = q.LockPasswordUser(ctx, id); err != nil {
		return fmt.Errorf("lock recovery account: %w", err)
	}
	current, err := q.GetPasswordCredential(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		normalized, err := auth.NormalizeUsername(*username)
		if err != nil {
			return errors.New("--username is required for an unconfigured account")
		}
		_, err = q.CreatePasswordCredential(ctx, db.CreatePasswordCredentialParams{UserID: id, Username: normalized, PasswordHash: hash, MustChangePassword: true})
		if err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if *username != "" && *username != current.Username {
			return errors.New("recovery cannot change an existing username")
		}
		_, err = q.ChangePasswordCredential(ctx, db.ChangePasswordCredentialParams{UserID: id, PasswordHash: hash, MustChangePassword: true, SessionVersion: current.SessionVersion})
		if err != nil {
			return err
		}
	}
	if _, err = auth.RevokePasswordCredentials(ctx, tx, id); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Password recovered. The user must change the temporary password at next login.")
	return nil
}

func readRecoveryPassword() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		// stty is supplied by the existing Linux/macOS host, and is deliberately
		// invoked without a shell. Failure aborts before reading any secret.
		disable := exec.Command("stty", "-echo")
		disable.Stdin = os.Stdin
		if err = disable.Run(); err != nil {
			return "", errors.New("cannot disable terminal echo; supply password over a protected stdin pipe")
		}
		defer func() {
			restore := exec.Command("stty", "echo")
			restore.Stdin = os.Stdin
			_ = restore.Run()
			fmt.Fprintln(os.Stderr)
		}()
		fmt.Fprint(os.Stderr, "Temporary password: ")
	}
	line, err := bufio.NewReader(io.LimitReader(os.Stdin, 515)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	password := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if err = auth.ValidatePassword(password); err != nil {
		return "", err
	}
	return password, nil
}
