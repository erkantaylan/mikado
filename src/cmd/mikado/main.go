// Command mikado is a task map for any goal: journeys, the quests toward
// them (GitHub issues, errands or petitions) and the side issues found along
// the way. `mikado serve`
// runs the local server (JSON API + dashboard); every other command is a thin
// client of that API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"mikado/internal/api"
	"mikado/internal/github"
	"mikado/internal/skill"
	"mikado/internal/store"
	"mikado/internal/web"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	defaultAddr   = "127.0.0.1:47291"
	defaultServer = "http://" + defaultAddr
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "version":
		err = versionCmd(args)
	case "serve":
		err = serve(args)
	case "skill":
		err = skillCmd(args)
	case "help", "-h", "--help":
		usage()
	default:
		if current, ok := aliases[cmd]; ok {
			cmd = current
		}
		run, ok := commands[cmd]
		if !ok {
			fmt.Fprintf(os.Stderr, "mikado: unknown command %q\n\n", cmd)
			usage()
			os.Exit(2)
		}
		err = run(args)
	}
	if err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "mikado %s: %s\n", cmd, ue.msg)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "mikado: %v\n", err)
		os.Exit(1)
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", defaultAddr, "address to listen on (loopback only)")
	data := fs.String("data", "", "data directory (default $MIKADO_DATA, else $XDG_DATA_HOME/mikado, else ~/.local/share/mikado)")
	var hosts listFlag
	fs.Var(&hosts, "allow-host", "also answer requests addressed to this host, e.g. mikado.home or *.ts.net (repeatable; added to $MIKADO_ALLOWED_HOSTS)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	// Commas or spaces separate hosts in the environment variable.
	hosts = append(strings.FieldsFunc(os.Getenv("MIKADO_ALLOWED_HOSTS"), func(r rune) bool { return r == ',' || unicode.IsSpace(r) }), hosts...)
	for i, h := range hosts {
		clean, err := store.CleanHost(h)
		if err != nil {
			return usageError{"--allow-host " + err.Error()}
		}
		hosts[i] = clean
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return usageError{fmt.Sprintf("--addr %q: %v", *addr, err)}
	}
	// There is no authentication yet, so the server never leaves this machine.
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return usageError{fmt.Sprintf("--addr %q: mikado has no authentication yet and only listens on loopback (127.0.0.1)", *addr)}
	}
	dir, err := dataDir(*data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(dir, "mikado.db"), &github.Client{})
	if err != nil {
		return err
	}
	defer st.Close()

	apiHandler, err := api.Handler(st, hosts...)
	if err != nil {
		return err
	}
	hs, err := st.Hosts(context.Background())
	if err != nil {
		return err
	}
	var stored []string
	for _, h := range hs {
		stored = append(stored, h.Name)
	}
	// Bind before saying we serve, so a taken port is the only thing reported.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("something is already listening on %s, probably the mikado service (`systemctl --user status mikado`); "+
				"to accept another host name, run `mikado hosts add NAME` instead of starting a second server", *addr)
		}
		return err
	}
	srv := &http.Server{
		Handler:           web.Handler(version, apiHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("mikado %s serving on http://%s (data: %s)", version, *addr, dir)
	if len(hosts) > 0 {
		log.Printf("also answering requests addressed to %s (--allow-host, $MIKADO_ALLOWED_HOSTS)", strings.Join(hosts, ", "))
	}
	if len(stored) > 0 {
		log.Printf("also answering requests addressed to %s (mikado hosts)", strings.Join(stored, ", "))
	}
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// versionCmd prints the CLI's version and the running server's. It never
// fails for want of a server: the version is worth knowing then too.
func versionCmd(args []string) error {
	cmd := newCommand("version")
	if _, err := cmd.parse(args, 0, 0, "[--server URL] [--json]"); err != nil {
		return err
	}
	cl := cmd.client()
	cl.hc.Timeout = 3 * time.Second
	var health struct {
		Version string `json:"version"`
	}
	_, err := cl.do("GET", "/api/health", nil, &health)
	if *cmd.json {
		out := map[string]any{"version": version, "server": cl.base, "serverVersion": nil}
		if err == nil {
			out["serverVersion"] = health.Version
		}
		return printJSON(out)
	}
	fmt.Println("mikado", version)
	if err != nil {
		fmt.Printf("server not reachable at %s\n", cl.base)
		return nil
	}
	fmt.Printf("server %s at %s\n", health.Version, cl.base)
	if health.Version != version {
		fmt.Println("the running server is a different build; if you just installed mikado, restart it: systemctl --user restart mikado")
	}
	return nil
}

// dataDir picks the data directory: the flag, else $MIKADO_DATA, else
// $XDG_DATA_HOME/mikado, else ~/.local/share/mikado.
func dataDir(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if d := os.Getenv("MIKADO_DATA"); d != "" {
		return filepath.Abs(d)
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "mikado"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "mikado"), nil
}

// skillCmd prints the agent guide, installs it where Claude Code finds it, or
// says where that is. It needs no server.
func skillCmd(args []string) error {
	fs := flag.NewFlagSet("skill", flag.ContinueOnError)
	dir := fs.String("dir", "", "directory to install SKILL.md into (default ~/.claude/skills/mikado)")
	force := fs.Bool("force", false, "replace a SKILL.md that mikado did not write")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	target := *dir
	if target == "" {
		if target, err = skill.DefaultDir(); err != nil {
			return err
		}
	}
	switch {
	case len(pos) == 0:
		fmt.Print(skill.Text())
	case len(pos) == 1 && pos[0] == "install":
		path, err := skill.Install(target, *force)
		if err != nil {
			return err
		}
		fmt.Printf("installed the mikado skill at %s\n", path)
	case len(pos) == 1 && pos[0] == "path":
		fmt.Println(filepath.Join(target, "SKILL.md"))
	default:
		return usageError{"usage: mikado skill [install [--dir DIR] [--force] | path [--dir DIR]]"}
	}
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: mikado <command> [args] [flags]

  A journey (J7) is a goal, drawn as a war table of quests ending in one crowning quest.
  A quest is an issue, an errand or a petition, named Q142; "Q5 requires Q3" means
  Q3 must be fulfilled first (Q3 opens Q5). Full glossary: mikado skill.

AI agents: run `+"`mikado skill`"+` before using mikado. It is the guide, glossary included.

server:
  serve [--addr ADDR] [--data DIR] [--allow-host HOST]
                                      run the API and dashboard (default `+defaultAddr+`);
                                      it answers localhost, *.localhost and accepted hosts
  hosts                               the host names it also answers, e.g. behind a
                                      reverse proxy (mikado.home, *.ts.net)
  hosts add HOST... / remove HOST...  accept a host name, or stop, right away; kept in the
                                      database (--allow-host, repeatable, and
                                      $MIKADO_ALLOWED_HOSTS still add hosts for one run)
  version                             print the CLI's version and the running server's

agents:
  skill                               print the guide to working with mikado (SKILL.md)
  skill install [--dir DIR] [--force] install it for Claude Code (~/.claude/skills/mikado)
  skill path [--dir DIR]              where install puts it

journeys (a journey is its crowning quest, every quest that one requires, and their
side quests; J is a journey key, J7):
  journey new "title" [--crown Q]     start a journey
  journey list [--all]                every journey at a glance (the atlas);
                                      --all includes archived journeys
  journey show J                      its war table: quests, their status and
                                      requirements, chronicle
  journey crown J Q                   make Q the journey's crowning quest
  journey set J [--title T] [--crown Q]
  journey archive J / unarchive J     put a journey away (off the atlas, its war table
                                      and quests untouched) / bring it back

quests are global, one per GitHub issue. Q is a quest key (Q142, Q-142, q142, 142), an
issue owner/repo#n, an issue URL, or a journey key for that journey's crowning quest (so
`+"`require Q5 J3`"+` makes Q5 wait on the whole journey J3, drawn there as one card).
A quest is in a journey once it is linked in.
  add owner/repo#N [link flags]       add a GitHub issue; if it is already on a war table,
                                      that quest is returned and the link flags applied to it
  errand "title" [link flags]         add a step not worth an issue
  petition "title" --on WHO [link flags]
                                      add something we await a reply on from someone
      link flags: --opens Q (repeatable)  --requires Q (repeatable)  --side-of Q
                  --crowns J  --found-on Q --reason "why"  --npc  --hero WHO
  show Q                              one quest: what it requires, what it opens, its journeys
  require Q PREREQ                    Q requires PREREQ fulfilled first (cycles are refused)
  unrequire Q PREREQ                  drop that requirement
  fulfil Q / unfulfil Q               mark an errand or petition fulfilled / not
                                      (issues are fulfilled by closing them on GitHub)
  take-up Q [--by WHO] / set-down Q   someone is on it right now (underway) / no longer
  abandon Q --reason "why"            won't do: stays on the war table, blocks nothing,
                                      counts in no total (its side quests are abandoned too)
  unabandon Q                         undo an abandon made here
  strike Q --reason "why"             gone for good, with its side quests (stays in the
                                      chronicle); prefer abandon
  set Q [--hero WHO] [--title T] [--npc=true|false]
  assign Q LOGIN... [--remove]        assign (or unassign) an issue on GitHub
  assignees owner/repo                who can be assigned in a repo

dashboard:
  open [J | Q] [--journey J] [--print]
                                      open the atlas, a journey's war table, or the war
                                      table of a journey holding Q with it selected
                                      (--print: URL)

every command but serve and skill takes --server URL (env MIKADO_SERVER, default
`+defaultServer+`) and --json to print the API's JSON instead of text.
`)
}
