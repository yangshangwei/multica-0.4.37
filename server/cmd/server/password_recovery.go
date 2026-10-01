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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
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
	reason := flags.String("reason", "", "Required reason for the audited password recovery")
	breakGlass := flags.Bool("break-glass", false, "Recover the last effective super administrator; temporarily leaves no usable administrator")
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
	*reason = strings.TrimSpace(*reason)
	if *reason == "" || len(*reason) > 1000 || !utf8.ValidString(*reason) || strings.ContainsRune(*reason, 0) {
		return errors.New("--reason must contain 1–1000 bytes of valid text")
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
	result, err := service.RecoverPasswordForDeploymentInTx(ctx, tx, service.DeploymentPasswordRecoveryParams{
		TargetUserID: id, Username: *username, PasswordHash: hash,
		Reason: *reason, RequestID: uuid.NewString(), BreakGlass: *breakGlass,
	})
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password recovery operation %s: %w", util.UUIDToString(result.Operation.ID), err)
	}
	fmt.Fprintln(os.Stdout, "Password recovered. The user must change the temporary password at next login.")
	if *breakGlass {
		fmt.Fprintln(os.Stdout, "No effective super administrator remains until password change completes. Then verify administrator access and configure a second named recovery administrator.")
	}
	fmt.Fprintf(os.Stdout, "Audit operation: %s\n", util.UUIDToString(result.Operation.ID))
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
