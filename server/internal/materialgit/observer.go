package materialgit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Observer exposes no remote mutation. Transport is supplied by the authorized
// runtime host; the server must never construct an observer with Git credentials.
type Observer interface {
	Observe(context.Context, int64) (Review, error)
	ReadFile(context.Context, string, string) (string, error)
	FindReview(context.Context, string, string) (int64, error)
	ObserveDecision(context.Context, int64, string, string) (string, error)
}
type observer struct {
	reader interface {
		Observe(context.Context, int64) (Review, error)
		ReadFile(context.Context, string, string) (string, error)
	}
	c *client
}

func (o *observer) Observe(ctx context.Context, n int64) (Review, error) {
	return o.reader.Observe(ctx, n)
}
func (o *observer) ReadFile(ctx context.Context, sha, path string) (string, error) {
	return o.reader.ReadFile(ctx, sha, path)
}

type readTransport struct{ next http.RoundTripper }

func (t readTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Body == nil {
			return nil, failure("observer_write_refused")
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 32001))
		if err != nil || len(b) > 32000 {
			return nil, failure("observer_write_refused")
		}
		var q struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(b, &q) != nil || (q.Query != classicRuleQuery && q.Query != gateQuery) {
			return nil, failure("observer_write_refused")
		}
		r.Body = io.NopCloser(strings.NewReader(string(b)))
	}
	r.Header.Del("Authorization")
	r.Header.Del("PRIVATE-TOKEN")
	return t.next.RoundTrip(r)
}
func NewObserver(provider, instance, repository string, transport http.RoundTripper) (Observer, error) {
	if transport == nil {
		return nil, failure("observer_transport_required")
	}
	p, err := newClient(Connection{Provider: provider, BaseURL: instance, Repository: repository, Token: "host-transport"}, &http.Client{Transport: readTransport{transport}}, false)
	if err != nil {
		return nil, err
	}
	var c *client
	switch v := p.(type) {
	case *githubProvider:
		c = v.client
	case *gitlabProvider:
		c = v.client
	}
	return &observer{reader: p, c: c}, nil
}
func (o *observer) FindReview(ctx context.Context, branch, base string) (int64, error) {
	if !validRef(branch) || !validRef(base) {
		return 0, failure("invalid_review")
	}
	github := o.c.connection.Provider == "github"
	q := url.Values{"state": {"all"}, "per_page": {"100"}}
	endpoint := o.c.repo + "/pulls"
	if github {
		q.Set("head", strings.Split(o.c.connection.Repository, "/")[0]+":"+branch)
	} else {
		endpoint = o.c.repo + "/merge_requests"
		q.Set("source_branch", branch)
		q.Set("scope", "all")
	}
	var found int64
	for page := 1; ; page++ {
		q.Set("page", strconv.Itoa(page))
		var rows []struct {
			Number int64 `json:"number"`
			IID    int64 `json:"iid"`
		}
		h, err := o.c.request(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil, &rows)
		if err != nil {
			return 0, err
		}
		for _, r := range rows {
			n := r.Number
			if !github {
				n = r.IID
			}
			if n <= 0 || found != 0 {
				return 0, failure("candidate_review_ambiguous")
			}
			found = n
		}
		next, err := nextPage(h, page, len(rows), github)
		if err != nil {
			return 0, err
		}
		if !next {
			break
		}
	}
	if found == 0 {
		// An empty review listing alone cannot establish absence: a previous publish
		// may have left a branch. First prove repository access, then exact ref absence.
		var repository map[string]json.RawMessage
		if _, err := o.c.request(ctx, http.MethodGet, o.c.repo, nil, &repository); err != nil {
			return 0, err
		}
		var err error
		switch p := o.reader.(type) {
		case *githubProvider:
			_, err = p.ref(ctx, branch)
		case *gitlabProvider:
			_, err = p.branch(ctx, branch)
		}
		if err == nil {
			return 0, failure("candidate_branch_unconfirmed")
		}
		if !isStatus(err, 404) {
			return 0, err
		}
		return 0, failure("candidate_effect_absent")
	}
	return found, nil
}
func (o *observer) ObserveDecision(ctx context.Context, number int64, marker, body string) (string, error) {
	if number <= 0 || marker == "" || !strings.Contains(body, marker) {
		return "", failure("invalid_decision_note")
	}
	github := o.c.connection.Provider == "github"
	endpoint := o.c.repo + "/issues/" + strconv.FormatInt(number, 10) + "/comments"
	if !github {
		endpoint = o.c.repo + "/merge_requests/" + strconv.FormatInt(number, 10) + "/notes"
	}
	found := ""
	for page := 1; ; page++ {
		var rows []struct {
			Body   string
			User   struct{ Login string }
			Author struct{ Username string }
		}
		h, err := o.c.request(ctx, http.MethodGet, endpoint+"?per_page=100&page="+strconv.Itoa(page), nil, &rows)
		if err != nil {
			return "", err
		}
		for _, r := range rows {
			if strings.Contains(r.Body, marker) {
				if r.Body != body || found != "" {
					return "", failure("decision_note_conflict")
				}
				found = r.User.Login
				if !github {
					found = r.Author.Username
				}
				if found == "" {
					return "", failure("decision_actor_missing")
				}
			}
		}
		next, err := nextPage(h, page, len(rows), github)
		if err != nil {
			return "", err
		}
		if !next {
			break
		}
	}
	if found == "" {
		return "", failure("decision_note_missing")
	}
	return found, nil
}
