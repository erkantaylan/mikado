package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"mikado/internal/store"
)

// openCmd opens the dashboard in the browser: the atlas, a journey's war
// table, or the war table of a journey holding a quest, with that quest
// selected.
func openCmd(args []string) error {
	cmd := newCommand("open")
	journey := cmd.fs.String("journey", "", "for a quest in several journeys: which journey's war table to open")
	printOnly := cmd.fs.Bool("print", false, "print the URL instead of opening a browser")
	pos, err := cmd.parse(args, 0, 1, "[J | Q] [--journey J] [--print]")
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
		if page, err = openTarget(cl, pos[0], *journey); err != nil {
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

// openTarget turns a journey key or a quest reference into a dashboard path.
func openTarget(cl *client, target, journey string) (string, error) {
	if id, ok := store.ParseJourneyID(target); ok {
		key := store.JourneyKey(id)
		if _, err := cl.do("GET", journeyPath(key), nil, nil); err != nil {
			return "", err
		}
		return "/journey/" + key, nil
	}

	id, err := resolve(cl, target)
	if err != nil {
		return "", err
	}
	var v store.CardView
	if _, err := cl.do("GET", cardPath(id), nil, &v); err != nil {
		return "", err
	}
	if len(v.Journeys) == 0 {
		return "", fmt.Errorf("%s is in no journey yet, so there is no war table to show it on", v.Card.Key)
	}
	key := v.Journeys[0].Key
	if journey != "" {
		want, ok := store.ParseJourneyID(journey)
		if !ok {
			return "", usageError{fmt.Sprintf("--journey %q is not a journey (want J7)", journey)}
		}
		key = ""
		for _, j := range v.Journeys {
			if j.Key == store.JourneyKey(want) {
				key = j.Key
			}
		}
		if key == "" {
			return "", fmt.Errorf("%s is not in journey %s", v.Card.Key, journey)
		}
	}
	return "/journey/" + url.PathEscape(key) + "?quest=" + url.QueryEscape(v.Card.Key), nil
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
