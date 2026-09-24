// Package github reads and edits GitHub issues by shelling out to the `gh`
// CLI. gh holds the token; mikado stores no credentials of its own.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ref names one issue: owner/repo#number.
type Ref struct {
	Owner  string
	Repo   string
	Number int
}

func (r Ref) String() string { return fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number) }

// Key is the case-insensitive identity of a ref (GitHub names are
// case-insensitive), used for caching and duplicate detection.
func (r Ref) Key() string { return strings.ToLower(r.String()) }

// Repository is owner/repo.
func (r Ref) Repository() string { return r.Owner + "/" + r.Repo }

var (
	refRe = regexp.MustCompile(`^([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)/([A-Za-z0-9._-]+)#([1-9][0-9]*)$`)
	urlRe = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/(?:issues|pull)/([1-9][0-9]*)/?$`)
	// nameRe matches a GitHub owner or repository name.
	nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// ParseRef accepts owner/repo#n or an issue URL on github.com.
func ParseRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	m := refRe.FindStringSubmatch(s)
	if m == nil {
		m = urlRe.FindStringSubmatch(s)
	}
	if m == nil || !nameRe.MatchString(m[2]) {
		return Ref{}, fmt.Errorf("%q is not an issue reference (want owner/repo#number)", s)
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return Ref{}, fmt.Errorf("%q: bad issue number", s)
	}
	return Ref{Owner: m[1], Repo: m[2], Number: n}, nil
}

// ValidRepo reports whether owner and repo look like GitHub names.
func ValidRepo(owner, repo string) bool {
	return nameRe.MatchString(owner) && nameRe.MatchString(repo)
}

// Issue is what mikado keeps about an issue. Ref carries GitHub's own casing.
type Issue struct {
	Ref   Ref
	Title string
	State string // "open" or "closed"
	// StateReason is GitHub's raw reason: COMPLETED, NOT_PLANNED, DUPLICATE,
	// REOPENED, or empty.
	StateReason string
	Assignees   []string
	URL         string
	IsPR        bool
}

// Client runs gh. The zero value uses "gh" from PATH.
type Client struct {
	Bin     string        // defaults to "gh"
	Timeout time.Duration // per gh invocation; defaults to 30s
}

// batchSize bounds how many issues one GraphQL query asks for.
const batchSize = 100

// Issues fetches many issues, across repos, with one `gh api graphql` call per
// batchSize refs. The result is keyed by Ref.Key(); refs that do not exist
// (or whose repo does not exist or is not visible) are absent from it.
func (c *Client) Issues(ctx context.Context, refs []Ref) (map[string]Issue, error) {
	out := make(map[string]Issue, len(refs))
	uniq := dedupe(refs)
	for start := 0; start < len(uniq); start += batchSize {
		end := min(start+batchSize, len(uniq))
		if err := c.issueBatch(ctx, uniq[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func dedupe(refs []Ref) []Ref {
	seen := map[string]bool{}
	var uniq []Ref
	for _, r := range refs {
		if !seen[r.Key()] {
			seen[r.Key()] = true
			uniq = append(uniq, r)
		}
	}
	return uniq
}

type gqlIssue struct {
	Typename  string `json:"__typename"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Reason    string `json:"stateReason"`
	URL       string `json:"url"`
	Number    int    `json:"number"`
	Assignees struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`
}

func (c *Client) issueBatch(ctx context.Context, refs []Ref, out map[string]Issue) error {
	// One repository alias per repo, one issue alias per number inside it.
	type repoGroup struct {
		owner, repo string
		refs        []Ref
	}
	var groups []*repoGroup
	byRepo := map[string]*repoGroup{}
	for _, r := range refs {
		k := strings.ToLower(r.Repository())
		g := byRepo[k]
		if g == nil {
			g = &repoGroup{owner: r.Owner, repo: r.Repo}
			byRepo[k] = g
			groups = append(groups, g)
		}
		g.refs = append(g.refs, r)
	}
	var q strings.Builder
	q.WriteString("query{")
	for gi, g := range groups {
		fmt.Fprintf(&q, "r%d:repository(owner:%s,name:%s){nameWithOwner ", gi, strconv.Quote(g.owner), strconv.Quote(g.repo))
		for ii, r := range g.refs {
			fmt.Fprintf(&q, "i%d:issueOrPullRequest(number:%d){__typename ...on Issue{number title state stateReason url assignees(first:50){nodes{login}}} ...on PullRequest{number url}} ", ii, r.Number)
		}
		q.WriteString("}")
	}
	q.WriteString("}")

	stdout, runErr := c.run(ctx, "api", "graphql", "-f", "query="+q.String())
	var resp struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	// gh exits non-zero when the response carries GraphQL errors, but still
	// prints the body; NOT_FOUND errors are an answer, not a failure.
	if err := json.Unmarshal(stdout, &resp); err != nil || resp.Data == nil {
		if runErr != nil {
			return runErr
		}
		return fmt.Errorf("gh api graphql: unexpected response: %.200s", stdout)
	}
	for _, e := range resp.Errors {
		if e.Type != "NOT_FOUND" {
			return fmt.Errorf("gh api graphql: %s", e.Message)
		}
	}
	for gi, g := range groups {
		raw := resp.Data[fmt.Sprintf("r%d", gi)]
		var repo map[string]json.RawMessage
		if len(raw) == 0 || json.Unmarshal(raw, &repo) != nil || repo == nil {
			continue // repository not found
		}
		var nameWithOwner string
		_ = json.Unmarshal(repo["nameWithOwner"], &nameWithOwner)
		owner, name, ok := strings.Cut(nameWithOwner, "/")
		if !ok {
			owner, name = g.owner, g.repo
		}
		for ii, r := range g.refs {
			var iss gqlIssue
			if raw := repo[fmt.Sprintf("i%d", ii)]; len(raw) == 0 || json.Unmarshal(raw, &iss) != nil || iss.Typename == "" {
				continue // issue not found
			}
			is := Issue{
				Ref:         Ref{Owner: owner, Repo: name, Number: r.Number},
				Title:       iss.Title,
				State:       strings.ToLower(iss.State),
				StateReason: iss.Reason,
				URL:         iss.URL,
				IsPR:        iss.Typename == "PullRequest",
				Assignees:   []string{},
			}
			for _, n := range iss.Assignees.Nodes {
				is.Assignees = append(is.Assignees, n.Login)
			}
			sort.Strings(is.Assignees)
			out[r.Key()] = is
		}
	}
	return nil
}

// Assign adds and removes assignees on an issue.
func (c *Client) Assign(ctx context.Context, ref Ref, add, remove []string) error {
	args := []string{"issue", "edit", strconv.Itoa(ref.Number), "-R", ref.Repository()}
	if len(add) > 0 {
		args = append(args, "--add-assignee", strings.Join(add, ","))
	}
	if len(remove) > 0 {
		args = append(args, "--remove-assignee", strings.Join(remove, ","))
	}
	_, err := c.run(ctx, args...)
	return err
}

// Assignees lists the users that can be assigned to issues in a repo.
func (c *Client) Assignees(ctx context.Context, owner, repo string) ([]string, error) {
	if !ValidRepo(owner, repo) {
		return nil, fmt.Errorf("%s/%s is not a repository name", owner, repo)
	}
	out, err := c.run(ctx, "api", fmt.Sprintf("repos/%s/%s/assignees", owner, repo), "--paginate", "--jq", ".[].login")
	if err != nil {
		return nil, err
	}
	logins := []string{}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			logins = append(logins, l)
		}
	}
	sort.Strings(logins)
	return logins, nil
}

// run executes gh and returns stdout. A failure carries gh's stderr.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "gh"
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		var exitErr *exec.ExitError
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			msg = "timed out"
		case msg == "" && !errors.As(err, &exitErr):
			msg = err.Error()
		case msg == "":
			msg = "exited " + strconv.Itoa(exitErr.ExitCode())
		}
		msg = strings.TrimPrefix(msg, "gh: ")
		return stdout.Bytes(), fmt.Errorf("gh %s: %s", args[0], firstLine(msg))
	}
	return stdout.Bytes(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
