// Command mikado groups GitHub issues from several repos under small goals
// (quests) and tracks the side issues found along the way. `mikado serve`
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
		fmt.Println("mikado", version)
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

  A quest is a goal, drawn as a chart of deeds (tasks) ending in one crowning deed.
  A deed is an issue, an errand or a petition, named M142; "M5 requires M3" means
  M3 must be fulfilled first (M3 opens M5). Full glossary: mikado skill.

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
  version                             print the version

agents:
  skill                               print the guide to working with mikado (SKILL.md)
  skill install [--dir DIR] [--force] install it for Claude Code (~/.claude/skills/mikado)
  skill path [--dir DIR]              where install puts it

quests (a quest is its crowning deed, every deed that one requires, and their side quests):
  quest new "title" [--slug S] [--crown D]   start a quest
  quest list [--all]                  every quest at a glance (the Quest Board);
                                      --all includes archived quests
  quest show SLUG                     its chart: deeds, their status and requirements, chronicle
  quest crown SLUG D                  make D the quest's crowning deed
  quest rename SLUG NEW-SLUG          change the slug
  quest set SLUG [--slug S] [--title T] [--crown D]
  quest archive SLUG / unarchive SLUG put a quest away (off the board, its chart and
                                      deeds untouched) / bring it back

deeds are global, one per GitHub issue. D is a deed id (M142, M-142, m142, 142), an
issue owner/repo#n or an issue URL. A deed is in a quest once it is linked in.
  add owner/repo#N [link flags]       add a GitHub issue; if it is already on the chart,
                                      that deed is returned and the link flags applied to it
  errand "title" [link flags]         add a step not worth an issue
  petition "title" --on WHO [link flags]
                                      add something we await a reply on from someone
      link flags: --opens D (repeatable)  --requires D (repeatable)  --side-of D
                  --crowns SLUG  --unearthed-on D --reason "why"  --npc  --hero WHO
  show D                              one deed: what it requires, what it opens, its quests
  require D PREREQ                    D requires PREREQ fulfilled first (cycles are refused)
  unrequire D PREREQ                  drop that requirement
  fulfil D / unfulfil D               mark an errand or petition fulfilled / not
                                      (issues are fulfilled by closing them on GitHub)
  take-up D [--by WHO] / set-down D   someone is on it right now (underway) / no longer
  abandon D --reason "why"            won't do: stays on the chart, blocks nothing, counts
                                      in no total (its side quests are abandoned too)
  unabandon D                         undo an abandon made here
  strike D --reason "why"             gone for good, with its side quests (stays in the
                                      chronicle); prefer abandon
  set D [--hero WHO] [--title T] [--npc=true|false]
  assign D LOGIN... [--remove]        assign (or unassign) an issue on GitHub
  assignees owner/repo                who can be assigned in a repo

dashboard:
  open [SLUG | D] [--quest SLUG] [--print]
                                      open the Quest Board, a quest's chart, or the chart
                                      of a quest holding D with it selected (--print: URL)

every command but serve and skill takes --server URL (env MIKADO_SERVER, default
`+defaultServer+`) and --json to print the API's JSON instead of text.
`)
}
