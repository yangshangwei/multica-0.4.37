package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/cli"
)

// `multica issue budget`: the commands a person uses to make an issue the root
// of a budget and to read what that root has observed (design 12.6). The HTTP
// shape they speak is frozen in design 12.9; cmd_issue_budget_wire.go reads it
// and cmd_issue_budget_print.go prints it.

var issueBudgetCmd = newIssueBudgetCommand()

const (
	budgetOutputTable = "table"
	budgetOutputJSON  = "json"
)

func newIssueBudgetCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "budget",
		Short: "Observe what an issue tree costs against a budget root (for people, not for agents)",
		Long: `A budget root makes one issue the root of a budget: the executions of that
issue and of the issues below it are observed against one limit.

These commands are for people. They need the credential of an owner or admin
of the workspace and are refused inside an agent task.

Every write takes --request-key, a UUID that you choose and keep: it names the
request. A write is sent once and never again on its own. When its outcome is
not known, ask by the same key with "multica issue budget operation"; if that
answers "unknown", run the very same command again with the same key and the
same flags.

A write exits with 0 only when its operation is committed. With any other exit
code the first line of the message says what happened: it starts with
"refused (" when the server refused the operation and nothing was written, and
with "outcome unknown: " when nothing says whether it was committed.

Amounts are US dollars with at most 10 digits after the point, or "none".`,
	}

	enable := &cobra.Command{
		Use:   "enable <issue>",
		Short: "Make an issue the root of a budget and start observing its tree",
		RunE:  runIssueBudgetEnable,
	}
	budgetWriteFlags(enable)
	budgetAmountFlags(enable)

	setLimit := &cobra.Command{
		Use:   "set-limit",
		Short: "Set or clear the limit and the reserve of each execution of a budget root",
		RunE:  runIssueBudgetSetLimit,
	}
	budgetRootFlag(setLimit)
	budgetWriteFlags(setLimit)
	budgetRevisionFlag(setLimit)
	budgetAmountFlags(setLimit)

	setMode := &cobra.Command{
		Use:   "set-mode",
		Short: "Switch the observation of a budget root off or on",
		RunE:  runIssueBudgetSetMode,
	}
	budgetRootFlag(setMode)
	budgetWriteFlags(setMode)
	budgetRevisionFlag(setMode)
	setMode.Flags().String("mode", "", "off or observe (required)")

	closeRoot := &cobra.Command{
		Use:   "close",
		Short: "Close a budget root",
		RunE:  runIssueBudgetClose,
	}
	budgetRootFlag(closeRoot)
	budgetWriteFlags(closeRoot)
	budgetRevisionFlag(closeRoot)

	reopen := &cobra.Command{
		Use:   "reopen",
		Short: "Reopen a closed budget root",
		RunE:  runIssueBudgetReopen,
	}
	budgetRootFlag(reopen)
	budgetWriteFlags(reopen)
	budgetRevisionFlag(reopen)

	show := &cobra.Command{
		Use:   "show (--root <id> | <issue>)",
		Short: "Show a budget root: its policy, its amounts and what is unknown",
		RunE:  runIssueBudgetShow,
	}
	show.Flags().String("root", "", "Budget root ID; or give an issue instead, and the root it belongs to is shown")

	operation := &cobra.Command{
		Use:   "operation (--root <id> | --issue <issue>) --request-key <key>",
		Short: "Ask what became of a write, by the key it was sent with",
		RunE:  runIssueBudgetOperation,
	}
	operation.Flags().String("root", "", "Budget root ID: asks about a set-limit, set-mode, close or reopen")
	operation.Flags().String("issue", "", "Issue: asks about the enable of this issue")
	operation.Flags().String("request-key", "", "The key the write was sent with (required)")

	observations := &cobra.Command{
		Use:   "observations",
		Short: "List what a budget root has observed, newest first",
		RunE:  runIssueBudgetObservations,
	}
	attempts := &cobra.Command{
		Use:   "attempts",
		Short: "List the executions of a budget root with their reservations, newest first",
		RunE:  runIssueBudgetAttempts,
	}
	for _, list := range []*cobra.Command{observations, attempts} {
		budgetRootFlag(list)
		list.Flags().String("limit", "", "Rows on one page, 1 to 100 (the server chooses when it is left out)")
		list.Flags().String("cursor", "", "Where the page starts: the cursor the page before printed")
	}

	command.AddCommand(enable, setLimit, setMode, closeRoot, reopen, show, operation, observations, attempts)
	for _, sub := range command.Commands() {
		// A person reads these answers, so the table is the default; json prints
		// the answer of the server as it was received.
		sub.Flags().String("output", budgetOutputTable, "Output format: table or json")
		// Arguments are checked by the command itself, after it has made sure
		// that a person runs it.
		sub.Args = cobra.ArbitraryArgs
	}
	return command
}

func budgetRootFlag(cmd *cobra.Command) {
	cmd.Flags().String("root", "", "Budget root ID (required)")
}

func budgetWriteFlags(cmd *cobra.Command) {
	cmd.Flags().String("request-key", "", "Your key for this request: a canonical lower-case UUID that you choose and keep (required)")
	cmd.Flags().String("reason", "", "Why this is done; it is recorded with the operation (required)")
	cmd.Flags().Bool("yes", false, "Confirm that the request may be sent (required)")
}

func budgetRevisionFlag(cmd *cobra.Command) {
	cmd.Flags().String("expect-revision", "", "The policy revision this change is based on, as \"show\" printed it (required)")
}

func budgetAmountFlags(cmd *cobra.Command) {
	cmd.Flags().String("limit", "", "Limit in US dollars, or none for no limit (required)")
	cmd.Flags().String("attempt-reserve", "", "Amount reserved for each execution in US dollars, or none (required)")
}

// Values of design 12.9 that these commands send or tell apart. Every other
// value of an answer is printed as it came.
const (
	budgetKindEnable   = "enable"
	budgetKindSetLimit = "set_limit"
	budgetKindSetMode  = "set_mode"
	budgetKindClose    = "close"
	budgetKindReopen   = "reopen"

	budgetModeOff     = "off"
	budgetModeObserve = "observe"

	budgetStateTombstoned = "tombstoned"

	budgetLedgerAvailable   = "available"
	budgetLedgerUnavailable = "unavailable"

	budgetKeyCommitted = "committed"
	budgetKeyUnknown   = "unknown"
)

// budgetStart is the first thing every subcommand does, the reads as much as
// the writes: it makes sure that a person runs the command, with a credential
// of their own and inside a workspace, before a flag is looked at or anything
// is sent. It returns the client and the output format.
func budgetStart(cmd *cobra.Command) (*cli.APIClient, string, error) {
	command := "issue budget " + cmd.Name()
	if err := requireHumanLocalCommand(command); err != nil {
		return nil, "", err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return nil, "", err
	}
	if err := budgetHumanCredential(client, command); err != nil {
		return nil, "", err
	}
	if !reportSnapshotUUID(client.WorkspaceID) {
		return nil, "", fmt.Errorf("%s needs the workspace as a canonical lower-case UUID; pass --workspace-id or set MULTICA_WORKSPACE_ID", command)
	}
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return nil, "", err
	}
	if output != budgetOutputTable && output != budgetOutputJSON {
		return nil, "", errors.New(`--output must be "table" or "json"`)
	}
	return client, output, nil
}

// budgetHumanCredential refuses every client that is not a person with a token
// of their own: no token, the token of a task, of a daemon or of the cloud,
// and a client that carries the identity of an agent or of a task. The
// snapshot command makes this refusal for its writes; here it holds for every
// subcommand.
func budgetHumanCredential(client *cli.APIClient, command string) error {
	switch {
	case client.Token == "":
		return fmt.Errorf(`%s requires a human credential, and no token is configured; sign in with "multica login"`, command)
	case strings.HasPrefix(client.Token, cli.TaskTokenPrefix), strings.HasPrefix(client.Token, "mdt_"),
		strings.HasPrefix(client.Token, auth.CloudPATPrefix), client.AgentID != "", client.TaskID != "":
		return fmt.Errorf("%s requires a human credential; the credential of an agent, a task, a daemon or the cloud is refused", command)
	}
	return nil
}

// Nothing is filled in for the operator. The functions below read what was
// typed: a flag that was left out is an error, never a default, and a value is
// checked in the form the server takes before anything is sent. No error of
// theirs shows a value the operator did not type, least of all a key.

// budgetTyped returns the value of a flag that must be given.
func budgetTyped(cmd *cobra.Command, name string) (string, error) {
	if !cmd.Flags().Changed(name) {
		return "", fmt.Errorf("--%s is required", name)
	}
	return cmd.Flags().GetString(name)
}

// budgetTypedID reads --root or --request-key: a canonical lower-case UUID
// that is not all zeros, checked the way the snapshot command checks its key.
func budgetTypedID(cmd *cobra.Command, name string) (string, error) {
	id, err := budgetTyped(cmd, name)
	if err != nil {
		return "", err
	}
	if !reportSnapshotUUID(id) {
		return "", fmt.Errorf("--%s must be a canonical lower-case UUID that is not all zeros", name)
	}
	return id, nil
}

// budgetTypedIntent reads the key and the reason of a write. The key is the
// operator's: this command never makes one up and never changes it. Of the
// reason only the encoding is checked here, because the two Go lines this
// command is built with write bytes that are not UTF-8 differently; what else
// a reason must be is the server's to say.
func budgetTypedIntent(cmd *cobra.Command) (key, reason string, err error) {
	if key, err = budgetTypedID(cmd, "request-key"); err != nil {
		return "", "", err
	}
	if reason, err = budgetTyped(cmd, "reason"); err != nil {
		return "", "", err
	}
	if !utf8.ValidString(reason) {
		return "", "", errors.New("--reason must be valid UTF-8")
	}
	return key, reason, nil
}

// budgetTypedRevision reads --expect-revision: a positive whole number in the
// form the wire carries it, without a sign or a leading zero.
func budgetTypedRevision(cmd *cobra.Command) (budget.Count, error) {
	text, err := budgetTyped(cmd, "expect-revision")
	if err != nil {
		return 0, err
	}
	revision, err := budget.ParseDecimal(text)
	if err != nil || revision <= 0 {
		return 0, errors.New(`--expect-revision must be a positive whole number without a sign or a leading zero: the policy revision that "show" printed`)
	}
	return budget.Count(revision), nil
}

// budgetTypedAmounts reads --limit and --attempt-reserve. Both must be given,
// as an amount or as none, so that no amount is set or cleared by leaving a
// flag out.
func budgetTypedAmounts(cmd *cobra.Command) (limit, reserve *budget.Ticks, err error) {
	if limit, err = budgetTypedAmount(cmd, "limit"); err != nil {
		return nil, nil, err
	}
	if reserve, err = budgetTypedAmount(cmd, "attempt-reserve"); err != nil {
		return nil, nil, err
	}
	return limit, reserve, nil
}

func budgetTypedAmount(cmd *cobra.Command, name string) (*budget.Ticks, error) {
	text, err := budgetTyped(cmd, name)
	if err != nil {
		return nil, err
	}
	amount, err := parseBudgetUSD(text)
	if err != nil {
		return nil, fmt.Errorf("--%s %w", name, err)
	}
	return amount, nil
}

// budgetTypedYes refuses a write that was not confirmed.
func budgetTypedYes(cmd *cobra.Command) error {
	if yes, err := cmd.Flags().GetBool("yes"); err != nil || !yes {
		return errors.New("--yes is required: it confirms that this request may be sent")
	}
	return nil
}

// budgetNoArguments refuses a positional argument on a command that is
// addressed by flags alone.
func budgetNoArguments(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%s takes no argument: what it needs is given by flags", cmd.Name())
	}
	return nil
}

func budgetIssuePath(issue string) string {
	return "/api/issues/" + url.PathEscape(issue) + "/budget"
}

func budgetRootPath(root string) string {
	return "/api/issue-budgets/" + url.PathEscape(root)
}

// budgetMaxAnswerBytes bounds what is read of one answer. A page of a hundred
// rows is a small part of it; an answer that is longer is not one of 12.9.
const budgetMaxAnswerBytes = 1 << 20

var errBudgetAnswerTooLong = errors.New("the answer is longer than this command reads")

// budgetAnswer is what came back for one request. A status of 0 means that no
// answer came back.
type budgetAnswer struct {
	status int
	body   []byte
}

// httpError is the answer as the error the rest of the CLI knows: main takes
// the exit code from its status, and --debug prints it.
func (a budgetAnswer) httpError(method, path string) *cli.HTTPError {
	body := a.body
	if len(body) > 4096 {
		body = body[:4096]
	}
	return &cli.HTTPError{Method: method, Path: path, StatusCode: a.status, Body: string(body)}
}

// budgetRequest sends one request and reads its answer, whatever its status.
// The request reaches the wire at most once. The helpers of cli.APIClient have
// no retry of their own either, but they follow redirects and keep the status
// and the body of an answer to themselves, and this command needs both to tell
// the outcome of a write. So the request is made here:
//
//   - a redirect is not followed: its answer is returned as it is;
//   - a write carries no GetBody, so the transport cannot send its body a
//     second time, and no Idempotency-Key header, which the transport would
//     read as leave to replay a POST on a connection that broke;
//   - it carries the workspace and the token of the operator and never the
//     identity of an agent or a task.
//
// An error with a status of 0 means that no answer came back. An error with a
// status means that the answer has that status and a body that broke off or is
// longer than this command reads.
func budgetRequest(ctx context.Context, client *cli.APIClient, method, path string, body []byte) (budgetAnswer, error) {
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, client.BaseURL+path, payload)
	if err != nil {
		return budgetAnswer{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.GetBody = nil
	}
	for name, value := range map[string]string{
		"X-Workspace-ID":    client.WorkspaceID,
		"X-Client-Platform": cli.ClientPlatform,
		"X-Client-Version":  cli.ClientVersion,
		"X-Client-OS":       cli.ClientOS,
	} {
		if value != "" {
			req.Header.Set(name, value)
		}
	}
	req.Header.Set("Authorization", "Bearer "+client.Token)
	httpClient := *client.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := httpClient.Do(req)
	if err != nil {
		return budgetAnswer{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, budgetMaxAnswerBytes+1))
	switch {
	case err != nil:
		return budgetAnswer{status: response.StatusCode}, err
	case len(data) > budgetMaxAnswerBytes:
		return budgetAnswer{status: response.StatusCode}, errBudgetAnswerTooLong
	}
	return budgetAnswer{status: response.StatusCode, body: data}, nil
}

// budgetRefusalMeanings is every code of design 12.3 with one line of what it
// means. A code that is not here is not a refusal this command knows.
var budgetRefusalMeanings = map[string]string{
	"invalid_request":          "the server does not accept the form of the request: the key, the reason, an amount or the mode",
	"budget_forbidden":         "budget roots are for a person who is an owner or admin of the workspace and can see the root issue",
	"not_found":                "no such issue or budget root is visible in this workspace",
	"not_eligible":             "this issue cannot be the root of a budget: only an ordinary issue can",
	"already_enabled":          "this issue is, or once was, the root issue of a budget root, and an issue is enabled only once",
	"root_conflict":            "this issue, or an issue below it, already belongs to another budget root",
	"parent_cycle":             "the parents of this issue form a loop",
	"operation_conflict":       "this request key was already used for a request with other content; one key names one request",
	"policy_revision_conflict": "the policy of this budget root has changed since the revision given with --expect-revision",
	"root_deleted":             "the root issue of this budget root was deleted, so the root cannot be reopened",
	"state_conflict":           "the budget root is already in the state this command asks for, so nothing would change",
}

const (
	budgetPolicyRevisionConflict = "policy_revision_conflict"
	budgetRefusedUnauthorized    = `refused (401): the server did not accept the credential; sign in with "multica login".`
)

// budgetRefusal reads an answer as a refusal by the budget handlers: a 4xx
// whose body is the error shape of 12.9 with a code of 12.3. Anything else is
// not one, whatever its status says.
func budgetRefusal(answer budgetAnswer) (code string, ok bool) {
	if answer.status < 400 || answer.status > 499 {
		return "", false
	}
	var refusal budgetErrorBody
	if err := understandBudgetAnswer(answer.body, &refusal); err != nil || refusal.Error.value != refusal.Code.value {
		return "", false
	}
	_, known := budgetRefusalMeanings[refusal.Code.value]
	return refusal.Code.value, known
}

// budgetWrite is one write, with what is needed to tell its outcome and to say
// how to ask about it afterwards.
type budgetWrite struct {
	kind  string // the operation_kind the result of this write has
	path  string
	body  any
	key   string // --request-key as it was typed
	root  string // --root; "" for an enable
	issue string // the issue of an enable; "" otherwise
}

// send sends the write once and reports one of the three outcomes of design
// 12.6: committed (printed, no error), refused, or unknown. Whatever the
// request came to, a second one is never sent: not the write again, and not a
// read to find out more.
func (w budgetWrite) send(cmd *cobra.Command, client *cli.APIClient, output string) error {
	body, err := json.Marshal(w.body)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	answer, err := budgetRequest(ctx, client, http.MethodPost, w.path, body)
	status := fmt.Sprintf("the server answered HTTP %d", answer.status)
	switch {
	case answer.status == http.StatusUnauthorized:
		// The credential is turned away before a handler sees the request.
		return cli.WithUserMessage(budgetRefusedUnauthorized+" Nothing was written.", answer.httpError(http.MethodPost, w.path))
	case err != nil && answer.status == 0:
		return w.unknown(cmd, "no answer came back from the server", err)
	case errors.Is(err, errBudgetAnswerTooLong):
		return w.unknown(cmd, status+" with more than this command reads", err)
	case err != nil:
		return w.unknown(cmd, status+", but its answer broke off before its end", err)
	case answer.status == http.StatusCreated, answer.status == http.StatusOK:
		return w.committed(cmd, answer, output)
	}
	if code, ok := budgetRefusal(answer); ok {
		text := fmt.Sprintf("refused (%s): %s. Nothing was written.", code, budgetRefusalMeanings[code])
		if code == budgetPolicyRevisionConflict {
			text += "\nRun \"" + w.showCommand(cmd) + "\" and decide again: this command does not look up the revision for you and does not send the request again."
		}
		return cli.WithUserMessage(text, answer.httpError(http.MethodPost, w.path))
	}
	// The cause is kept for --debug as a plain error: an error of the HTTP kind
	// would give this outcome the exit code of a refusal.
	return w.unknown(cmd, status+", which does not say whether the operation was committed", errors.New(answer.httpError(http.MethodPost, w.path).Error()))
}

// committed handles a 201 or a 200. The operation is committed only when the
// body is understood and belongs to this request; a 2xx that does not say so
// in the shape of 12.9 leaves the outcome unknown.
func (w budgetWrite) committed(cmd *cobra.Command, answer budgetAnswer, output string) error {
	status := fmt.Sprintf("the server answered HTTP %d", answer.status)
	var written budgetWriteAnswer
	err := understandBudgetAnswer(answer.body, &written)
	result := &written.Result.value
	if err == nil {
		err = budgetResultUnderstood(result)
	}
	switch {
	case err != nil:
		return w.unknown(cmd, status+", but its answer was not understood: "+err.Error(), err)
	case answer.status == http.StatusOK && !written.Replayed.value:
		return w.unknown(cmd, status+" without saying that this key had already committed", errBudgetNotThisRequest)
	case result.OperationKind.value != w.kind:
		return w.unknown(cmd, status+" with the result of another kind of operation", errBudgetNotThisRequest)
	case w.root != "" && result.BudgetRootID.value != w.root:
		return w.unknown(cmd, status+" with the result of another budget root", errBudgetNotThisRequest)
	}
	if output == budgetOutputJSON {
		err = budgetPrintReceived(cmd, answer.body)
	} else {
		err = printBudgetWritten(cmd.OutOrStdout(), &written)
	}
	if err != nil {
		return fmt.Errorf("the operation is committed, but its result could not be printed: %w", err)
	}
	return nil
}

// unknown reports the third outcome: nothing says whether the operation is
// committed. The text sends the operator to the query by the key they typed
// and never to another key. why is this command's own wording: what a
// transport or a body says about itself may read as a verdict ("failed") that
// nobody has given, and may hold an ID that is not the operator's.
func (w budgetWrite) unknown(cmd *cobra.Command, why string, cause error) error {
	byKey := []string{"--issue", w.issue}
	if w.root != "" {
		byKey = []string{"--root", w.root}
	}
	text := "outcome unknown: " + why + ".\n" +
		"This command sent the request at most once and does not send it again; the operation may be committed or not.\n" +
		"Ask by the original key:\n" +
		"  " + budgetCommandLine(cmd, "operation", append(byKey, "--request-key", w.key)...) + "\n" +
		"Keep the original --request-key and the exact flags of this command: if the query answers \"unknown\", run this very same command again."
	return cli.WithUserMessage(text, cause)
}

// showCommand is the command that shows the policy this write was aimed at.
func (w budgetWrite) showCommand(cmd *cobra.Command) string {
	if w.root != "" {
		return budgetCommandLine(cmd, "show", "--root", w.root)
	}
	return budgetCommandLine(cmd, "show", w.issue)
}

// budgetCommandLine spells a command of this group so that the operator can
// run it as it is printed: each word quoted where a shell needs it, followed by
// the flags that choose the profile, the server and the workspace when the
// operator typed them for the command that prints this one.
func budgetCommandLine(cmd *cobra.Command, sub string, words ...string) string {
	line := []string{"multica", "issue", "budget", sub}
	for _, word := range words {
		line = append(line, budgetShellWord(word))
	}
	for _, name := range []string{"profile", "server-url", "workspace-id"} {
		if flag := cmd.Flag(name); flag != nil && flag.Changed {
			line = append(line, "--"+name, budgetShellWord(flag.Value.String()))
		}
	}
	return strings.Join(line, " ")
}

// budgetShellWord leaves a word bare when a POSIX shell takes it as it is and
// puts it in single quotes otherwise. Its bare set is the one of shellQuoteArg
// and the colon, so that a server URL is printed the way it was typed.
func budgetShellWord(word string) string {
	if word == "" || strings.ContainsFunc(word, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:", r))
	}) {
		return shellQuoteArg(word)
	}
	return word
}

func runIssueBudgetEnable(cmd *cobra.Command, args []string) error {
	client, output, err := budgetStart(cmd)
	if err != nil {
		return err
	}
	if len(args) != 1 || args[0] == "" {
		return errors.New("enable takes exactly one issue, by its identifier or its ID")
	}
	issue := args[0]
	key, reason, err := budgetTypedIntent(cmd)
	if err != nil {
		return err
	}
	limit, reserve, err := budgetTypedAmounts(cmd)
	if err != nil {
		return err
	}
	if err := budgetTypedYes(cmd); err != nil {
		return err
	}
	return budgetWrite{
		kind:  budgetKindEnable,
		path:  budgetIssuePath(issue),
		body:  budgetEnableRequest{RequestKey: key, Reason: reason, LimitTicks: limit, AttemptReserveTicks: reserve},
		key:   key,
		issue: issue,
	}.send(cmd, client, output)
}

// runBudgetRootWrite runs one of the four operations on a budget root. extra
// reads the flags that only this kind has and returns the body to send.
func runBudgetRootWrite(cmd *cobra.Command, args []string, kind string, extra func(budgetOperationRequest) (any, error)) error {
	client, output, err := budgetStart(cmd)
	if err != nil {
		return err
	}
	if err := budgetNoArguments(cmd, args); err != nil {
		return err
	}
	root, err := budgetTypedID(cmd, "root")
	if err != nil {
		return err
	}
	key, reason, err := budgetTypedIntent(cmd)
	if err != nil {
		return err
	}
	revision, err := budgetTypedRevision(cmd)
	if err != nil {
		return err
	}
	body, err := extra(budgetOperationRequest{Kind: kind, RequestKey: key, Reason: reason, ExpectedPolicyRevision: revision})
	if err != nil {
		return err
	}
	if err := budgetTypedYes(cmd); err != nil {
		return err
	}
	return budgetWrite{kind: kind, path: budgetRootPath(root) + "/operations", body: body, key: key, root: root}.send(cmd, client, output)
}

func runIssueBudgetSetLimit(cmd *cobra.Command, args []string) error {
	return runBudgetRootWrite(cmd, args, budgetKindSetLimit, func(request budgetOperationRequest) (any, error) {
		limit, reserve, err := budgetTypedAmounts(cmd)
		return budgetSetLimitRequest{budgetOperationRequest: request, LimitTicks: limit, AttemptReserveTicks: reserve}, err
	})
}

func runIssueBudgetSetMode(cmd *cobra.Command, args []string) error {
	return runBudgetRootWrite(cmd, args, budgetKindSetMode, func(request budgetOperationRequest) (any, error) {
		mode, err := budgetTyped(cmd, "mode")
		if err == nil && mode != budgetModeOff && mode != budgetModeObserve {
			err = errors.New(`--mode must be "off" or "observe"`)
		}
		return budgetSetModeRequest{budgetOperationRequest: request, Mode: mode}, err
	})
}

func runIssueBudgetClose(cmd *cobra.Command, args []string) error {
	return runBudgetRootWrite(cmd, args, budgetKindClose, func(request budgetOperationRequest) (any, error) { return request, nil })
}

func runIssueBudgetReopen(cmd *cobra.Command, args []string) error {
	return runBudgetRootWrite(cmd, args, budgetKindReopen, func(request budgetOperationRequest) (any, error) { return request, nil })
}

// budgetRead sends one read and returns the body of a 200. Everything else is
// an error that says which of three things happened: the server refused, it
// answered something that is not an answer of the budget API, or no answer
// came back. A read changes nothing, so none of them is reported with the
// words of a write.
func budgetRead(cmd *cobra.Command, client *cli.APIClient, path string) ([]byte, error) {
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	answer, err := budgetRequest(ctx, client, http.MethodGet, path, nil)
	status := answer.httpError(http.MethodGet, path)
	switch {
	case answer.status == http.StatusUnauthorized:
		return nil, cli.WithUserMessage(budgetRefusedUnauthorized, status)
	case errors.Is(err, errBudgetAnswerTooLong):
		return nil, fmt.Errorf("the server answered HTTP %d with more than this command reads", answer.status)
	case err != nil:
		// What the connection reported is kept for --debug, the way the rest of
		// the CLI keeps it: it names the URL, and its wording is the transport's.
		return nil, cli.WithUserMessage("no answer came back from the server, or not a whole one; nothing was read. Run the command again, with --debug to see what the connection reported.",
			&cli.NetworkError{Kind: cli.KindNetworkOffline, Op: http.MethodGet + " " + path, Err: err})
	case answer.status == http.StatusOK:
		return answer.body, nil
	}
	if code, ok := budgetRefusal(answer); ok {
		return nil, cli.WithUserMessage(fmt.Sprintf("refused (%s): %s.", code, budgetRefusalMeanings[code]), status)
	}
	return nil, cli.WithUserMessage(fmt.Sprintf("the server answered HTTP %d, which is not an answer of the budget API; nothing was read", answer.status), status)
}

// budgetNotUnderstood is the error of a read whose answer cannot be used. No
// part of such an answer is printed, in either output format.
func budgetNotUnderstood(err error) error {
	return fmt.Errorf("the answer of the server was not understood: %w; nothing of it is shown", err)
}

var (
	errBudgetAnotherRoot    = errors.New("the server answered about another budget root than the one that was asked for; nothing of the answer is shown")
	errBudgetNotThisRequest = errors.New("the answer does not belong to this request")
)

// budgetResultUnderstood checks what 12.9 says about a result beyond the
// presence of each field: the result of an enable carries its activation time.
func budgetResultUnderstood(result *budgetOperationResult) error {
	if result.OperationKind.value == budgetKindEnable && !result.ActivatedAt.present {
		return &budgetAnswerError{field: "result.activated_at", problem: "is missing"}
	}
	return nil
}

// budgetPrintReceived prints an answer as it was received, byte for byte, so
// that a field a later server adds is not lost on the way.
func budgetPrintReceived(cmd *cobra.Command, body []byte) error {
	_, err := cmd.OutOrStdout().Write(body)
	return err
}

func runIssueBudgetShow(cmd *cobra.Command, args []string) error {
	client, output, err := budgetStart(cmd)
	if err != nil {
		return err
	}
	byRoot := cmd.Flags().Changed("root")
	switch {
	case len(args) > 1 || len(args) == 1 && args[0] == "":
		return errors.New("show takes one issue, by its identifier or its ID")
	case byRoot == (len(args) == 1):
		return errors.New("show needs either --root <id> or one issue, and not both")
	}

	// Asked by an issue, the first answer only says which root to ask for.
	var binding *budgetIssueBinding
	var root string
	if byRoot {
		if root, err = budgetTypedID(cmd, "root"); err != nil {
			return err
		}
	} else {
		if binding, err = budgetReadBinding(cmd, client, args[0]); err != nil {
			return err
		}
		root = binding.BudgetRootID.value
	}

	body, err := budgetRead(cmd, client, budgetRootPath(root))
	if err != nil {
		return err
	}
	var shown budgetShow
	if err := understandBudgetAnswer(body, &shown); err != nil {
		return budgetNotUnderstood(err)
	}
	switch {
	case shown.BudgetRootID.value != root:
		return errBudgetAnotherRoot
	case shown.LedgerState.value == budgetLedgerAvailable && shown.Amounts.null:
		return budgetNotUnderstood(&budgetAnswerError{field: "amounts", problem: "is null although the ledger is available"})
	}
	if output == budgetOutputJSON {
		return budgetPrintReceived(cmd, body)
	}
	return printBudgetShow(cmd.OutOrStdout(), binding, &shown)
}

// budgetReadBinding asks which budget root an issue belongs to. The root it
// names goes into the path of the next request, so it is used only when it is
// the ID of a root.
func budgetReadBinding(cmd *cobra.Command, client *cli.APIClient, issue string) (*budgetIssueBinding, error) {
	body, err := budgetRead(cmd, client, budgetIssuePath(issue))
	var status *cli.HTTPError
	if errors.As(err, &status) && status.StatusCode == http.StatusNotFound {
		// The budget handlers and the loader of the issue in front of them
		// both answer 404, and neither tells the two cases apart.
		return nil, cli.WithUserMessage(fmt.Sprintf("issue %s belongs to no budget root, or is not visible in this workspace", issue), status)
	}
	if err != nil {
		return nil, err
	}
	var binding budgetIssueBinding
	if err := understandBudgetAnswer(body, &binding); err != nil {
		return nil, budgetNotUnderstood(err)
	}
	if !reportSnapshotUUID(binding.BudgetRootID.value) {
		return nil, budgetNotUnderstood(&budgetAnswerError{field: "budget_root_id", problem: "is not the ID of a budget root"})
	}
	return &binding, nil
}

func runIssueBudgetOperation(cmd *cobra.Command, args []string) error {
	client, output, err := budgetStart(cmd)
	if err != nil {
		return err
	}
	if err := budgetNoArguments(cmd, args); err != nil {
		return err
	}
	byRoot, byIssue := cmd.Flags().Changed("root"), cmd.Flags().Changed("issue")
	if byRoot == byIssue {
		return errors.New("operation needs either --root <id> or --issue <issue>, and not both: --root asks about an operation on a budget root, --issue about the enable of an issue")
	}
	var root, path string
	if byRoot {
		if root, err = budgetTypedID(cmd, "root"); err != nil {
			return err
		}
		path = budgetRootPath(root)
	} else {
		issue, err := cmd.Flags().GetString("issue")
		if err != nil || issue == "" {
			return errors.New("--issue must not be empty")
		}
		path = budgetIssuePath(issue)
	}
	key, err := budgetTypedID(cmd, "request-key")
	if err != nil {
		return err
	}

	body, err := budgetRead(cmd, client, path+"/operations/"+url.PathEscape(key))
	if err != nil {
		return err
	}
	// The state says which of the two answers of 12.9 this is. A state this
	// command does not know is neither of them.
	var head budgetOperationState
	if err := understandBudgetAnswer(body, &head); err != nil {
		return budgetNotUnderstood(err)
	}
	var committed budgetOperationCommitted
	switch head.State.value {
	case budgetKeyCommitted:
		err := understandBudgetAnswer(body, &committed)
		if err == nil {
			err = budgetResultUnderstood(&committed.Result.value)
		}
		if err != nil {
			return budgetNotUnderstood(err)
		}
		if root != "" && committed.Result.value.BudgetRootID.value != root {
			return errBudgetAnotherRoot
		}
	case budgetKeyUnknown:
		var unknown budgetOperationUnknown
		if err := understandBudgetAnswer(body, &unknown); err != nil {
			return budgetNotUnderstood(err)
		}
	default:
		return budgetNotUnderstood(&budgetAnswerError{field: "state", problem: `is neither "committed" nor "unknown"`})
	}
	if output == budgetOutputJSON {
		return budgetPrintReceived(cmd, body)
	}
	if head.State.value == budgetKeyUnknown {
		return printBudgetOperationUnknown(cmd.OutOrStdout())
	}
	return printBudgetOperationCommitted(cmd.OutOrStdout(), &committed)
}

// budgetListPage reads what the two lists share and asks for one page. It
// returns the body and what the command of the next page repeats: the root,
// and the size of the page when one was typed.
func budgetListPage(cmd *cobra.Command, args []string) (body []byte, output string, next []string, err error) {
	client, output, err := budgetStart(cmd)
	if err != nil {
		return nil, "", nil, err
	}
	if err := budgetNoArguments(cmd, args); err != nil {
		return nil, "", nil, err
	}
	root, err := budgetTypedID(cmd, "root")
	if err != nil {
		return nil, "", nil, err
	}
	next = []string{"--root", root}
	query := url.Values{}
	if cmd.Flags().Changed("limit") {
		limit, _ := cmd.Flags().GetString("limit")
		if rows, err := budget.ParseDecimal(limit); err != nil || rows < 1 || rows > 100 {
			return nil, "", nil, errors.New("--limit must be a whole number from 1 to 100")
		}
		query.Set("limit", limit)
		next = append(next, "--limit", limit)
	}
	if cmd.Flags().Changed("cursor") {
		cursor, _ := cmd.Flags().GetString("cursor")
		if cursor == "" {
			return nil, "", nil, errors.New("--cursor must not be empty: leave it out for the first page")
		}
		query.Set("cursor", cursor)
	}
	// The cursor is a value of the query, so it is escaped as one: a path
	// escape would let "+" and "&" through.
	path := budgetRootPath(root) + "/" + cmd.Name()
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	body, err = budgetRead(cmd, client, path)
	return body, output, next, err
}

func runIssueBudgetObservations(cmd *cobra.Command, args []string) error {
	body, output, next, err := budgetListPage(cmd, args)
	if err != nil {
		return err
	}
	var page budgetObservationPage
	if err := understandBudgetAnswer(body, &page); err != nil {
		return budgetNotUnderstood(err)
	}
	if output == budgetOutputJSON {
		return budgetPrintReceived(cmd, body)
	}
	return printBudgetObservations(cmd.OutOrStdout(), &page, budgetNextPage(cmd, next, page.NextCursor.pointer()))
}

func runIssueBudgetAttempts(cmd *cobra.Command, args []string) error {
	body, output, next, err := budgetListPage(cmd, args)
	if err != nil {
		return err
	}
	var page budgetAttemptPage
	if err := understandBudgetAnswer(body, &page); err != nil {
		return budgetNotUnderstood(err)
	}
	if output == budgetOutputJSON {
		return budgetPrintReceived(cmd, body)
	}
	return printBudgetAttempts(cmd.OutOrStdout(), &page, budgetNextPage(cmd, next, page.NextCursor.pointer()))
}

// budgetNextPage is the last line of a list: the command that fetches the next
// page, or that there is none. The cursor is the server's text. It is put into
// a command only when all of it can be shown; quoting cannot make a text that
// moves the cursor of a terminal safe to print. An empty cursor is not put into
// one either: --cursor refuses it, so the command would not run.
func budgetNextPage(cmd *cobra.Command, repeat []string, cursor *string) string {
	switch {
	case cursor == nil:
		return "no further page"
	case budgetText(*cursor) != *cursor:
		return "next page: there is one, but its position cannot be put into a command here; read it with --output json"
	}
	return "next page: " + budgetCommandLine(cmd, cmd.Name(), append(repeat, "--cursor", *cursor)...)
}
