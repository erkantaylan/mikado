package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"mikado/internal/github"
	"mikado/internal/store"
)

// commands are the client commands; each is a thin wrapper over the API.
// Quests are global, so quest commands take no journey.
var commands = map[string]func([]string) error{
	"journey":  journeyCmd,
	"add":      func(a []string) error { return addCmd("add", store.KindIssue, a) },
	"errand":   func(a []string) error { return addCmd("errand", store.KindErrand, a) },
	"petition": func(a []string) error { return addCmd("petition", store.KindAwaiting, a) },
	"show":     showCmd,
	"strike":   strikeCmd,
	"require":  func(a []string) error { return requireCmd("require", a) },
	"unrequire": func(a []string) error {
		return requireCmd("unrequire", a)
	},
	"fulfil": func(a []string) error {
		return flagCmd("fulfil", "fulfilled", a, func(p *store.CardPatch) { p.Done = ptr(true) })
	},
	"unfulfil": func(a []string) error {
		return flagCmd("unfulfil", "unfulfilled", a, func(p *store.CardPatch) { p.Done = ptr(false) })
	},
	"abandon": abandonCmd,
	"unabandon": func(a []string) error {
		return flagCmd("unabandon", "no longer abandoned", a, func(p *store.CardPatch) { p.Cancelled = ptr(false) })
	},
	"take-up": takeUpCmd,
	"set-down": func(a []string) error {
		return flagCmd("set-down", "set down", a, func(p *store.CardPatch) { p.Working = ptr(false) })
	},
	"set":       setCmd,
	"assign":    assignCmd,
	"assignees": assigneesCmd,
	"open":      openCmd,
	"hosts":     hostsCmd,
}

// aliases are the commands' earlier names (and a spelling or two). They still
// work but are left out of the help, which lists only the names above.
var aliases = map[string]string{
	"await":     "petition",
	"done":      "fulfil",
	"undone":    "unfulfil",
	"fulfill":   "fulfil",
	"unfulfill": "unfulfil",
	"cancel":    "abandon",
	"uncancel":  "unabandon",
	"remove":    "strike",
	"start":     "take-up",
	"stop":      "set-down",
	"need":      "require",
	"unneed":    "unrequire",
}

func ptr[T any](v T) *T { return &v }

func journeyPath(key string) string { return "/api/journeys/" + url.PathEscape(key) }

func cardPath(id int64, rest ...string) string {
	return "/api/cards/" + strings.Join(append([]string{strconv.FormatInt(id, 10)}, rest...), "/")
}

// resolve turns a quest reference into an id: Q142, Q-142, q142 and 142
// locally; an issue ref, a github.com URL or a journey key (that journey's
// crowning quest) by asking the server.
func resolve(c *client, ref string) (int64, error) {
	if id, ok := store.ParseCardID(ref); ok {
		return id, nil
	}
	path := "/api/cards/" + url.PathEscape(strings.TrimSpace(ref))
	if r, err := github.ParseRef(ref); err == nil {
		path = "/api/cards/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Repo) + "%23" + strconv.Itoa(r.Number)
	} else if strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, "/#?") {
		return 0, usageError{fmt.Sprintf("%q is not a quest (want Q142, owner/repo#n or a journey key like J7)", ref)}
	}
	var v store.CardView
	if _, err := c.do("GET", path, nil, &v); err != nil {
		return 0, err
	}
	return v.Card.ID, nil
}

func resolveAll(c *client, refs []string) ([]int64, error) {
	var out []int64
	for _, ref := range refs {
		id, err := resolve(c, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func resolveOptional(c *client, ref string) (*int64, error) {
	if ref == "" {
		return nil, nil
	}
	id, err := resolve(c, ref)
	return &id, err
}

// The API's status and state values are plain (done, locked, …); people read
// these words instead.
func statusWord(c *store.Card) string {
	switch c.Status {
	case store.StatusDone:
		return "fulfilled"
	case store.StatusAvailable:
		return "open"
	case store.StatusLocked:
		return fmt.Sprintf("sealed · %d to go", c.OpenBefore)
	case store.StatusAwaiting:
		return "awaiting reply"
	case store.StatusCancelled:
		return "abandoned"
	}
	return c.Status
}

func journeyStateWord(state string) string {
	switch state {
	case store.JourneyComplete:
		return "fulfilled"
	case store.JourneyCancelled:
		return "abandoned"
	}
	return state
}

// journeyWord is the journey's state, marked when it is archived.
func journeyWord(state, archivedAt string) string {
	if archivedAt != "" {
		return journeyStateWord(state) + ", archived"
	}
	return journeyStateWord(state)
}

func journeyCmd(args []string) error {
	if len(args) == 0 {
		return usageError{"usage: mikado journey new|list|show|crown|set|archive|unarchive"}
	}
	sub, args := args[0], args[1:]
	switch sub {
	case "new":
		cmd := newCommand("journey new")
		crown := cmd.fs.String("crown", "", "its crowning quest Q")
		pos, err := cmd.parse(args, 1, 1, `"title" [--crown Q]`)
		if err != nil {
			return err
		}
		cl := cmd.client()
		body := map[string]any{"title": pos[0]}
		if *crown != "" {
			id, err := resolve(cl, *crown)
			if err != nil {
				return err
			}
			body["final"] = id
		}
		var j store.JourneySummary
		if _, err := cl.do("POST", "/api/journeys", body, &j); err != nil {
			return err
		}
		if *cmd.json {
			return printJSON(j)
		}
		fmt.Printf("journey %s created: %s\n", j.Key, j.Title)
		if *crown == "" {
			fmt.Fprintf(os.Stderr, "it has no crowning quest yet — crown one with `mikado journey crown %s Q` or `mikado add … --crowns %s`\n", j.Key, j.Key)
		}
		return nil
	case "list", "ls":
		cmd := newCommand("journey list")
		all := cmd.fs.Bool("all", false, "include archived journeys")
		if _, err := cmd.parse(args, 0, 0, "[--all] [--json]"); err != nil {
			return err
		}
		var every []store.JourneySummary
		rep, err := cmd.client().do("GET", "/api/journeys", nil, &every)
		if err != nil {
			return err
		}
		qs := every
		if !*all {
			qs = slices.DeleteFunc(slices.Clone(every), func(j store.JourneySummary) bool { return j.ArchivedAt != "" })
		}
		if *cmd.json {
			return printJSON(qs)
		}
		if hidden := len(every) - len(qs); hidden > 0 {
			defer fmt.Fprintf(os.Stderr, "%d archived journey(s) not shown (--all to include them)\n", hidden)
		}
		if len(qs) == 0 {
			if len(every) == 0 {
				fmt.Println(`no journeys yet — start one with: mikado journey new "title"`)
			}
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "KEY\tSTATE\tMAIN QUEST\tACHIEVEMENTS\tOPEN\tAWAITING REPLY\tUNDERWAY\tABANDONED\tHEROES\tTITLE")
		for _, q := range qs {
			fmt.Fprintf(tw, "%s\t%s\t%d/%d\t%d/%d\t%d\t%d\t%d\t%d\t%s\t%s\n", q.Key, journeyWord(q.State, q.ArchivedAt), q.Main.Done, q.Main.Total,
				q.Achievements.Done, q.Achievements.Total, q.Available, q.Awaiting, q.InProgress, q.Cancelled,
				strings.Join(q.Heroes, ","), q.Title)
		}
		tw.Flush()
		if w := rep.header.Get("X-Mikado-GitHub"); w != "" {
			fmt.Fprintln(os.Stderr, "github:", w)
		}
		return nil
	case "show":
		cmd := newCommand("journey show")
		pos, err := cmd.parse(args, 1, 1, "J [--json]")
		if err != nil {
			return err
		}
		var v store.JourneyView
		if _, err := cmd.client().do("GET", journeyPath(pos[0]), nil, &v); err != nil {
			return err
		}
		if *cmd.json {
			return printJSON(v)
		}
		showJourney(&v)
		return nil
	case "crown":
		cmd := newCommand("journey crown")
		pos, err := cmd.parse(args, 2, 2, "J Q")
		if err != nil {
			return err
		}
		cl := cmd.client()
		id, err := resolve(cl, pos[1])
		if err != nil {
			return err
		}
		return patchJourney(cmd, pos[0], map[string]any{"final": id})
	case "archive", "unarchive":
		cmd := newCommand("journey " + sub)
		pos, err := cmd.parse(args, 1, 1, "J")
		if err != nil {
			return err
		}
		return patchJourney(cmd, pos[0], map[string]any{"archived": sub == "archive"})
	case "set":
		cmd := newCommand("journey set")
		title := cmd.fs.String("title", "", "new title")
		crown := cmd.fs.String("crown", "", "its crowning quest Q")
		pos, err := cmd.parse(args, 1, 1, "J [--title T] [--crown Q]")
		if err != nil {
			return err
		}
		body := map[string]any{}
		if *title != "" {
			body["title"] = *title
		}
		if *crown != "" {
			id, err := resolve(cmd.client(), *crown)
			if err != nil {
				return err
			}
			body["final"] = id
		}
		if len(body) == 0 {
			return usageError{"nothing to set: pass --title or --crown"}
		}
		return patchJourney(cmd, pos[0], body)
	default:
		return usageError{fmt.Sprintf("unknown journey command %q (new, list, show, crown, set, archive, unarchive)", sub)}
	}
}

func patchJourney(cmd *command, key string, body map[string]any) error {
	var j store.JourneySummary
	if _, err := cmd.client().do("PATCH", journeyPath(key), body, &j); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(j)
	}
	fmt.Printf("journey %s: %s [%s]\n", j.Key, j.Title, journeyWord(j.State, j.ArchivedAt))
	return nil
}

// showJourney prints a journey as text: the crowning quest, then the main
// quest line, then side quests, each with status, people, what it requires
// and other journeys, then the recent chronicle.
func showJourney(v *store.JourneyView) {
	var main, side, mainDone, sideDone, cancelled int
	for _, c := range v.Cards {
		switch {
		case c.Cancelled:
			cancelled++
		case c.SideOf != nil:
			side++
			if c.Done {
				sideDone++
			}
		default:
			main++
			if c.Done {
				mainDone++
			}
		}
	}
	fmt.Printf("%s — %s [%s]\n", v.Journey.Key, v.Journey.Title, journeyWord(v.Journey.State, v.Journey.ArchivedAt))
	byID := map[int64]*store.Card{}
	for i := range v.Cards {
		byID[v.Cards[i].ID] = &v.Cards[i]
	}
	if v.Journey.FinalCardID == nil {
		fmt.Printf("crowning quest: none yet — crown one with `mikado journey crown %s Q`\n", v.Journey.Key)
	} else {
		fmt.Printf("main quest %d/%d fulfilled · achievements %d/%d · %d abandoned\n", mainDone, main, sideDone, side, cancelled)
		c := byID[*v.Journey.FinalCardID]
		fmt.Printf("crowning quest: %s %s [%s]\n", c.Key, cardName(c), statusWord(c))
	}
	requires := map[int64][]string{}
	for _, n := range v.Needs {
		requires[n.From] = append(requires[n.From], store.Key(n.To))
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	line := func(c *store.Card) {
		mark := " "
		if c.Final {
			mark = "♛"
		}
		extra := cardDetails(c)
		if rs := requires[c.ID]; len(rs) > 0 {
			extra = append(extra, "requires "+strings.Join(rs, " "))
		}
		if len(c.AlsoIn) > 0 {
			var keys []string
			for _, j := range c.AlsoIn {
				keys = append(keys, j.Key)
			}
			extra = append(extra, "also in "+strings.Join(keys, ", "))
		}
		name := nameWithWork(c)
		if q := c.Crowns; q != nil {
			// Another journey, folded into this one: one line, with its progress.
			name = fmt.Sprintf("journey %s  %d/%d  %s", q.Key, q.Done, q.Total, q.Title)
			extra = extra[:0]
			if q.Working > 0 {
				extra = append(extra, fmt.Sprintf("%d underway", q.Working))
			}
			if q.ArchivedAt != "" {
				extra = append(extra, "archived")
			}
			if rs := requires[c.ID]; len(rs) > 0 {
				extra = append(extra, "requires "+strings.Join(rs, " "))
			}
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%s\t%s\n", mark, c.Key, statusWord(c), name, strings.Join(extra, " · "))
	}
	if len(v.Cards) > 0 {
		fmt.Println()
		for i := range v.Cards {
			if v.Cards[i].SideOf == nil {
				line(&v.Cards[i])
			}
		}
		tw.Flush()
	}
	if hasSide(v.Cards) {
		fmt.Println("\nside quests:")
		for i := range v.Cards {
			if v.Cards[i].SideOf != nil {
				line(&v.Cards[i])
			}
		}
		tw.Flush()
	}
	if n := len(v.Log); n > 0 {
		fmt.Println("\nchronicle:")
		for _, e := range v.Log[max(0, n-6):] {
			fmt.Printf("  %s  %s\n", e.At, e.Text)
		}
	}
	if v.GitHub != "" {
		fmt.Fprintln(os.Stderr, "github:", v.GitHub)
	}
}

func hasSide(cards []store.Card) bool {
	for _, c := range cards {
		if c.SideOf != nil {
			return true
		}
	}
	return false
}

// cardDetails lists what is worth saying about a quest besides its name.
func cardDetails(c *store.Card) []string {
	var extra []string
	if c.Cancelled {
		switch {
		case c.CancelReason != "":
			extra = append(extra, "abandoned: "+c.CancelReason)
		case c.StateReason != "":
			extra = append(extra, "closed on GitHub as "+strings.ToLower(strings.ReplaceAll(c.StateReason, "_", " ")))
		}
	}
	if len(c.Assignees) > 0 {
		extra = append(extra, "@"+strings.Join(c.Assignees, " @"))
	}
	if c.Owner != "" {
		extra = append(extra, "hero "+c.Owner)
	}
	if c.Kind == store.KindAwaiting {
		if c.Status == "awaiting" || c.Status == "locked" {
			extra = append(extra, "awaiting reply from "+c.WaitingOn+" since "+c.Since)
		} else {
			extra = append(extra, "petitioned "+c.WaitingOn+" on "+c.Since)
		}
	}
	if c.SideOf != nil {
		extra = append(extra, "side quest on "+store.Key(*c.SideOf))
	}
	if c.FoundWhile != nil {
		extra = append(extra, "found on "+store.Key(*c.FoundWhile))
	}
	if c.NPC {
		extra = append(extra, "NPC")
	}
	return extra
}

func cardName(c *store.Card) string {
	switch c.Kind {
	case store.KindIssue:
		if c.Title == c.Ref {
			return c.Ref
		}
		return c.Ref + "  " + c.Title
	case store.KindAwaiting:
		return "(petition) " + c.Title
	default:
		return "(errand) " + c.Title
	}
}

func nameWithWork(c *store.Card) string {
	name := cardName(c)
	if c.Working {
		who := c.WorkingBy
		if who == "" {
			who = "since " + c.WorkingSince
		}
		name += " (underway: " + who + ")"
	}
	return name
}

func printCard(c *store.Card, verb string) {
	fmt.Printf("%s %s: %s [%s]\n", c.Key, verb, nameWithWork(c), statusWord(c))
}

func showCmd(args []string) error {
	cmd := newCommand("show")
	pos, err := cmd.parse(args, 1, 1, "Q [--json]")
	if err != nil {
		return err
	}
	cl := cmd.client()
	id, err := resolve(cl, pos[0])
	if err != nil {
		return err
	}
	var v store.CardView
	if _, err := cl.do("GET", cardPath(id), nil, &v); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(v)
	}
	c := &v.Card
	fmt.Printf("%s [%s] %s\n", c.Key, statusWord(c), nameWithWork(c))
	if c.URL != "" {
		fmt.Println("  " + c.URL)
	}
	if d := cardDetails(c); len(d) > 0 {
		fmt.Println("  " + strings.Join(d, " · "))
	}
	if c.Reason != "" && !c.Cancelled {
		fmt.Println("  why: " + c.Reason)
	}
	if q := c.Crowns; q != nil {
		fmt.Printf("  crowns journey %s (%s): %d/%d fulfilled, %s\n", q.Key, q.Title, q.Done, q.Total, journeyWord(q.State, q.ArchivedAt))
	}
	var journeys []string
	for _, j := range v.Journeys {
		journeys = append(journeys, j.Key+" ("+j.Title+")")
	}
	if len(journeys) == 0 {
		fmt.Println("  in no journey — link it with `mikado require` or `mikado journey crown`")
	} else {
		fmt.Println("  in: " + strings.Join(journeys, ", "))
	}
	for _, group := range []struct {
		title string
		cards []store.Card
	}{{"requires", v.Needs}, {"opens", v.NeededBy}, {"side quests", v.SideQuests}} {
		if len(group.cards) == 0 {
			continue
		}
		fmt.Printf("  %s:\n", group.title)
		for i := range group.cards {
			g := &group.cards[i]
			fmt.Printf("    %s [%s] %s\n", g.Key, statusWord(g), nameWithWork(g))
		}
	}
	if v.GitHub != "" {
		fmt.Fprintln(os.Stderr, "github:", v.GitHub)
	}
	return nil
}

func addCmd(name, kind string, args []string) error {
	cmd := newCommand(name)
	var requires, opens listFlag
	cmd.fs.Var(&requires, "requires", "the new quest requires quest Q fulfilled first (repeatable)")
	cmd.fs.Var(&opens, "opens", "the new quest opens quest Q: Q requires it (repeatable)")
	foundOn := cmd.fs.String("found-on", "", "quest Q this was found on")
	reason := cmd.fs.String("reason", "", "why it was added")
	sideOf := cmd.fs.String("side-of", "", "make it a side quest on quest Q (optional, never blocks)")
	crowns := cmd.fs.String("crowns", "", "make it the crowning quest of journey J")
	npc := cmd.fs.Bool("npc", false, "mark it as an NPC (drawn red on the chart)")
	hero := cmd.fs.String("hero", "", "who is responsible for it (free text)")
	cmd.alias("needs", "requires")
	cmd.alias("needed-by", "opens")
	cmd.alias("found-while", "found-on")
	cmd.alias("final-of", "crowns")
	cmd.alias("owner", "hero")
	var on *string
	what := `"title" [flags]`
	switch kind {
	case store.KindIssue:
		what = "owner/repo#N [flags]"
	case store.KindAwaiting:
		on = cmd.fs.String("on", "", "who the petition awaits a reply from (required)")
		what = `"title" --on WHO [flags]`
	}
	pos, err := cmd.parse(args, 1, 1, what)
	if err != nil {
		return err
	}
	cl := cmd.client()
	in := store.NewCard{Kind: kind, Reason: *reason, NPC: *npc, Owner: *hero, FinalOf: *crowns}
	if kind == store.KindIssue {
		r, err := github.ParseRef(pos[0])
		if err != nil {
			return usageError{err.Error()}
		}
		in.Ref = r.String()
	} else {
		in.Title = pos[0]
	}
	if on != nil {
		if *on == "" {
			return usageError{"--on WHO is required: who does the petition await a reply from?"}
		}
		in.WaitingOn = *on
	}
	if in.Needs, err = resolveAll(cl, requires); err != nil {
		return err
	}
	if in.NeededBy, err = resolveAll(cl, opens); err != nil {
		return err
	}
	if in.FoundWhile, err = resolveOptional(cl, *foundOn); err != nil {
		return err
	}
	if in.SideOf, err = resolveOptional(cl, *sideOf); err != nil {
		return err
	}
	var c store.Card
	rep, err := cl.do("POST", "/api/cards", in, &c)
	if err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(c)
	}
	verb := "added"
	if in.FoundWhile != nil {
		verb = "found on " + store.Key(*in.FoundWhile)
	}
	if rep.status == http.StatusOK {
		verb = "already on the chart"
		if len(requires)+len(opens) > 0 || *sideOf != "" || *crowns != "" {
			verb += ", links applied"
		}
	}
	printCard(&c, verb)
	if len(c.AlsoIn) == 0 {
		fmt.Fprintf(os.Stderr, "%s is in no journey yet — link it with --opens or `mikado require`\n", c.Key)
	} else {
		var keys []string
		for _, j := range c.AlsoIn {
			keys = append(keys, j.Key)
		}
		fmt.Printf("  in: %s\n", strings.Join(keys, ", "))
	}
	return nil
}

func strikeCmd(args []string) error {
	cmd := newCommand("strike")
	reason := cmd.fs.String("reason", "", "why it is struck (required)")
	pos, err := cmd.parse(args, 1, 1, `Q --reason "why"`)
	if err != nil {
		return err
	}
	if *reason == "" {
		return usageError{`--reason "why" is required`}
	}
	cl := cmd.client()
	id, err := resolve(cl, pos[0])
	if err != nil {
		return err
	}
	if _, err := cl.do("DELETE", cardPath(id), map[string]string{"reason": *reason}, nil); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(map[string]any{"removed": id, "key": store.Key(id)})
	}
	fmt.Printf("%s struck from the record (with its side quests)\n", store.Key(id))
	return nil
}

func requireCmd(name string, args []string) error {
	cmd := newCommand(name)
	pos, err := cmd.parse(args, 2, 2, "Q PREREQ")
	if err != nil {
		return err
	}
	cl := cmd.client()
	ids, err := resolveAll(cl, pos)
	if err != nil {
		return err
	}
	n := store.Need{From: ids[0], To: ids[1]}
	method := "POST"
	if name == "unrequire" {
		method = "DELETE"
	}
	if _, err := cl.do(method, "/api/needs", n, nil); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(n)
	}
	if name == "unrequire" {
		fmt.Printf("%s no longer requires %s\n", store.Key(n.From), store.Key(n.To))
	} else {
		fmt.Printf("%s requires %s (%s opens %s)\n", store.Key(n.From), store.Key(n.To), store.Key(n.To), store.Key(n.From))
	}
	return nil
}

func patch(cmd *command, ref string, p store.CardPatch, verb string) error {
	cl := cmd.client()
	id, err := resolve(cl, ref)
	if err != nil {
		return err
	}
	var c store.Card
	if _, err := cl.do("PATCH", cardPath(id), p, &c); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(c)
	}
	printCard(&c, verb)
	return nil
}

// flagCmd is a quest command that takes only Q and sets one field.
func flagCmd(name, verb string, args []string, set func(*store.CardPatch)) error {
	cmd := newCommand(name)
	pos, err := cmd.parse(args, 1, 1, "Q")
	if err != nil {
		return err
	}
	var p store.CardPatch
	set(&p)
	return patch(cmd, pos[0], p, verb)
}

func abandonCmd(args []string) error {
	cmd := newCommand("abandon")
	reason := cmd.fs.String("reason", "", "why it won't be done (required)")
	pos, err := cmd.parse(args, 1, 1, `Q --reason "why"`)
	if err != nil {
		return err
	}
	if *reason == "" {
		return usageError{`--reason "why" is required`}
	}
	return patch(cmd, pos[0], store.CardPatch{Cancelled: ptr(true), CancelReason: reason}, "abandoned")
}

func takeUpCmd(args []string) error {
	cmd := newCommand("take-up")
	by := cmd.fs.String("by", "", "who takes it up (free text)")
	pos, err := cmd.parse(args, 1, 1, "Q [--by WHO]")
	if err != nil {
		return err
	}
	p := store.CardPatch{Working: ptr(true)}
	verb := "taken up"
	if *by != "" {
		p.WorkingBy = by
		verb += " by " + *by
	}
	return patch(cmd, pos[0], p, verb)
}

func setCmd(args []string) error {
	cmd := newCommand("set")
	hero := cmd.fs.String("hero", "", `hero (free text; "-" leaves it with no hero)`)
	title := cmd.fs.String("title", "", "new title (errands and petitions)")
	npc := cmd.fs.String("npc", "", "true or false")
	cmd.alias("owner", "hero")
	pos, err := cmd.parse(args, 1, 1, "Q [--hero WHO] [--title T] [--npc=true|false]")
	if err != nil {
		return err
	}
	var p store.CardPatch
	set := map[string]bool{}
	cmd.fs.Visit(func(f *flag.Flag) { set[cmd.canonical(f.Name)] = true })
	if set["hero"] {
		o := *hero
		if o == "-" {
			o = ""
		}
		p.Owner = &o
	}
	if set["title"] {
		p.Title = title
	}
	if set["npc"] {
		b, err := strconv.ParseBool(*npc)
		if err != nil {
			return usageError{fmt.Sprintf("--npc wants true or false, not %q", *npc)}
		}
		p.NPC = &b
	}
	if !set["hero"] && !set["title"] && !set["npc"] {
		return usageError{"nothing to set: pass --hero, --title or --npc (a journey's crowning quest: mikado journey crown)"}
	}
	return patch(cmd, pos[0], p, "updated")
}

func assignCmd(args []string) error {
	cmd := newCommand("assign")
	remove := cmd.fs.Bool("remove", false, "unassign the logins instead")
	pos, err := cmd.parse(args, 2, -1, "Q LOGIN... [--remove]")
	if err != nil {
		return err
	}
	cl := cmd.client()
	id, err := resolve(cl, pos[0])
	if err != nil {
		return err
	}
	body := map[string][]string{"add": pos[1:], "remove": {}}
	verb := "assigned on GitHub"
	if *remove {
		body["add"], body["remove"] = []string{}, pos[1:]
		verb = "unassigned on GitHub"
	}
	var c store.Card
	if _, err := cl.do("POST", cardPath(id, "assignees"), body, &c); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(c)
	}
	printCard(&c, verb)
	return nil
}

func assigneesCmd(args []string) error {
	cmd := newCommand("assignees")
	pos, err := cmd.parse(args, 1, 1, "owner/repo")
	if err != nil {
		return err
	}
	owner, repo, ok := strings.Cut(pos[0], "/")
	if !ok || !github.ValidRepo(owner, repo) {
		return usageError{fmt.Sprintf("%q is not owner/repo", pos[0])}
	}
	var users []string
	if _, err := cmd.client().do("GET", "/api/repos/"+owner+"/"+repo+"/assignees", nil, &users); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(users)
	}
	for _, u := range users {
		fmt.Println(u)
	}
	return nil
}

// acceptedHost is an entry of GET /api/hosts.
type acceptedHost struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	AddedAt string `json:"addedAt,omitempty"`
}

// hostsCmd lists, adds or removes the host names the server answers besides
// localhost. Changes apply at once and are kept in the database.
func hostsCmd(args []string) error {
	sub := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "list", "ls":
		cmd := newCommand("hosts")
		if _, err := cmd.parse(args, 0, 0, "[--json]"); err != nil {
			return err
		}
		return listHosts(cmd)
	case "add", "remove", "rm":
		cmd := newCommand("hosts " + sub)
		pos, err := cmd.parse(args, 1, -1, "NAME...")
		if err != nil {
			return err
		}
		cl := cmd.client()
		for _, name := range pos {
			if sub == "add" {
				var h acceptedHost
				rep, err := cl.do("POST", "/api/hosts", map[string]string{"name": name}, &h)
				if err != nil {
					return err
				}
				if *cmd.json {
					continue
				}
				switch {
				case h.Source == "flag":
					fmt.Printf("%s is already accepted (--allow-host or $MIKADO_ALLOWED_HOSTS)\n", h.Name)
				case rep.status == http.StatusCreated:
					fmt.Printf("%s accepted\n", h.Name)
				default:
					fmt.Printf("%s is already accepted\n", h.Name)
				}
				continue
			}
			if _, err := cl.do("DELETE", "/api/hosts/"+url.PathEscape(name), nil, nil); err != nil {
				return err
			}
			if !*cmd.json {
				fmt.Printf("%s no longer accepted\n", name)
			}
		}
		if *cmd.json {
			return listHosts(cmd)
		}
		return nil
	default:
		return usageError{fmt.Sprintf("unknown hosts command %q (add, remove; none lists them)", sub)}
	}
}

func listHosts(cmd *command) error {
	var hs []acceptedHost
	if _, err := cmd.client().do("GET", "/api/hosts", nil, &hs); err != nil {
		return err
	}
	if *cmd.json {
		return printJSON(hs)
	}
	if len(hs) == 0 {
		fmt.Println("only localhost, *.localhost and loopback IPs — accept another name with: mikado hosts add NAME")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tSOURCE\tADDED")
	for _, h := range hs {
		source, added := "stored", h.AddedAt
		if h.Source == "flag" {
			source, added = "--allow-host / $MIKADO_ALLOWED_HOSTS", "(this run)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Name, source, added)
	}
	tw.Flush()
	fmt.Fprintln(os.Stderr, "localhost, *.localhost and loopback IPs are always accepted")
	return nil
}
