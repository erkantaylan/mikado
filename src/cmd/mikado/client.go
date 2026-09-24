package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// usageError is a mistake in how a command was called (exit status 2).
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// client talks to a running `mikado serve`.
type client struct {
	base string
	hc   *http.Client
}

func newClient(server string) *client {
	return &client{base: strings.TrimRight(server, "/"), hc: &http.Client{Timeout: 60 * time.Second}}
}

// reply is what a successful call returned besides its body.
type reply struct {
	status int
	header http.Header
}

// do sends body as JSON and decodes the response into out (if non-nil). An
// API error comes back as its {error} message.
func (c *client) do(method, path string, body, out any) (reply, error) {
	var none reply
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return none, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return none, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return none, fmt.Errorf("cannot reach mikado at %s (%v) — is `mikado serve` running?", c.base, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return none, err
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return none, errors.New(e.Error)
		}
		return none, fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return none, fmt.Errorf("%s %s: bad response: %v", method, path, err)
		}
	}
	return reply{status: resp.StatusCode, header: resp.Header}, nil
}

// listFlag is a repeatable string flag.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

// command is a flag set with the flags every client command shares.
type command struct {
	fs     *flag.FlagSet
	server *string
	json   *bool
	// aliases maps a flag's earlier name to its current one. Aliases work
	// but are left out of the -h listing.
	aliases map[string]string
}

func newCommand(name string) *command {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	server := os.Getenv("MIKADO_SERVER")
	if server == "" {
		server = defaultServer
	}
	return &command{
		fs:      fs,
		server:  fs.String("server", server, "mikado server URL (env MIKADO_SERVER)"),
		json:    fs.Bool("json", false, "print JSON"),
		aliases: map[string]string{},
	}
}

// alias lets the flag --current also be given as --old, sharing its value.
func (c *command) alias(old, current string) {
	f := c.fs.Lookup(current)
	c.fs.Var(f.Value, old, f.Usage)
	c.aliases[old] = current
}

// canonical is the current name of a flag given by any of its names.
func (c *command) canonical(name string) string {
	if n, ok := c.aliases[name]; ok {
		return n
	}
	return name
}

func (c *command) client() *client { return newClient(*c.server) }

// parse parses flags wherever they appear among the arguments and returns
// the positional ones, checking there are between min and max of them
// (max < 0: no limit).
func (c *command) parse(args []string, min, max int, what string) ([]string, error) {
	pos, err := parseFlags(c.fs, args, c.aliases)
	if err != nil {
		return nil, err
	}
	if len(pos) < min || (max >= 0 && len(pos) > max) {
		return nil, usageError{"usage: mikado " + c.fs.Name() + " " + what}
	}
	return pos, nil
}

// parseArgs lets flags follow positional arguments (the flag package stops
// at the first one). Everything after "--" is positional.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	return parseFlags(fs, args, nil)
}

// parseFlags is parseArgs for a flag set with aliases, which -h leaves out.
func parseFlags(fs *flag.FlagSet, args []string, hidden map[string]string) ([]string, error) {
	fs.SetOutput(io.Discard)
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				shown := flag.NewFlagSet(fs.Name(), flag.ContinueOnError)
				fs.VisitAll(func(f *flag.Flag) {
					if _, alias := hidden[f.Name]; !alias {
						shown.Var(f.Value, f.Name, f.Usage)
						shown.Lookup(f.Name).DefValue = f.DefValue
					}
				})
				shown.SetOutput(os.Stderr)
				shown.PrintDefaults()
				os.Exit(0)
			}
			return nil, usageError{err.Error()}
		}
		rest := fs.Args()
		// fs.Parse consumed a "--" if the arg before rest is "--".
		if n := len(args) - len(rest); n > 0 && args[n-1] == "--" {
			return append(pos, rest...), nil
		}
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
