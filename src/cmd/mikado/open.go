package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"mikado/internal/github"
	"mikado/internal/store"
)

// openCmd opens the dashboard in the browser: the Quest Board, a quest's chart,
// or the chart of a quest holding a deed, with that deed selected.
func openCmd(args []string) error {
	cmd := newCommand("open")
	quest := cmd.fs.String("quest", "", "for a deed in several quests: which quest's chart to open")
	printOnly := cmd.fs.Bool("print", false, "print the URL instead of opening a browser")
	pos, err := cmd.parse(args, 0, 1, "[SLUG | D] [--quest SLUG] [--print]")
	if err != nil {
		return err
	}
	cl := cmd.client()
	// A dead page is worse than an error: make sure the server answers first.
	if _, err := cl.do("GET", "/api/health", nil, nil); err != nil {
		return err
	}

	page := "/"
	if len(pos) == 1 {
		if page, err = openTarget(cl, pos[0], *quest); err != nil {
			return err
		}
	}
	link := cl.base + page
	if *printOnly || *cmd.json {
		fmt.Println(link)
		return nil
	}
	if err := browse(link); err != nil {
		return fmt.Errorf("could not open a browser (%v); the page is %s", err, link)
	}
	fmt.Println("opened", link)
	return nil
}

// openTarget turns a quest slug or a deed reference into a dashboard path.
func openTarget(cl *client, target, quest string) (string, error) {
	_, isCard := store.ParseCardID(target)
	if _, err := github.ParseRef(target); err == nil {
		isCard = true
	}
	if !isCard {
		if _, err := cl.do("GET", questPath(target), nil, nil); err != nil {
			return "", err
		}
		return "/quest/" + url.PathEscape(strings.ToLower(target)), nil
	}

	id, err := resolve(cl, target)
	if err != nil {
		return "", err
	}
	var v store.CardView
	if _, err := cl.do("GET", cardPath(id), nil, &v); err != nil {
		return "", err
	}
	if len(v.Quests) == 0 {
		return "", fmt.Errorf("%s is in no quest yet, so there is no chart to show it on", v.Card.Key)
	}
	slug := v.Quests[0].Slug
	if quest != "" {
		slug = ""
		for _, q := range v.Quests {
			if strings.EqualFold(q.Slug, quest) {
				slug = q.Slug
			}
		}
		if slug == "" {
			return "", fmt.Errorf("%s is not in quest %q", v.Card.Key, quest)
		}
	}
	return "/quest/" + url.PathEscape(slug) + "?deed=" + url.QueryEscape(v.Card.Key), nil
}

// browse hands a URL to the desktop's browser without waiting for it.
func browse(link string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", link)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	default:
		c = exec.Command("xdg-open", link)
	}
	return c.Start()
}
