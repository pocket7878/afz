package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sort"

	"github.com/renstrom/fuzzysearch/fuzzy"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
)

var (
	root     string
	cmd      []string
	rawOut   []byte
	outLines []string
	w        *acme.Win
	PLAN9    = os.Getenv("PLAN9")
)

const (
	debug = false
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: afz cmd \n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		usage()
	}

	root, _ = os.Getwd()
	cmd = args[0:]

	err := initWindow()

	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		return
	}

	for word := range events() {
		if len(word) >= 5 && word[0:5] == "Reset" {
			if err = doReset(); err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
				return
			}
			continue
		}
		if len(word) >= 6 && word[0:6] == "Search" {
			q := strings.TrimSpace(word[7:len(word)])
			doSearch(q)
			continue
		}
		if len(word) >= 5 && word[0:5] == "Fuzzy" {
			q := strings.TrimSpace(word[6:len(word)])
			doFuzzySearch(q)
			continue
		}
		onLook(word)
	}
}

func initWindow() error {
	var err error = nil
	w, err = acme.New()
	if err != nil {
		return err
	}

	title := root + "/" + "-afz"
	w.Name(title)
	tag := "Reset Search Fuzzy"
	w.Write("tag", []byte(tag))

	err = printCmdResult()
	if err != nil {
		return err
	}

	return nil
}

func printCmdResult() error {
	var err error = nil
	rawOut, err = exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return err
	}
	w.Write("body", rawOut)

	acc := make([]string, 0)
	for _, line := range bytes.Split(rawOut, []byte("\n")) {
		if debug {
			fmt.Fprintf(os.Stderr, "line: %v\n", line)
		}
		acc = append(acc, string(line))
	}
	outLines = acc
	return nil
}

func clear() error {
	err := w.Addr("0,$")
	if err != nil {
		return err
	}
	w.Write("data", []byte(""))
	return nil
}

func doReset() error {
	var err error = nil
	err = clear()
	if err != nil {
		return err
	}
	err = printCmdResult()
	if err != nil {
		return err
	}
	return nil
}

func doSearch(q string) {
	if debug {
		fmt.Fprintf(os.Stderr, "Search: %s\n", q)
		fmt.Fprintf(os.Stderr, "%v\n", outLines)
	}
	res := make([]string, 0)
	for _, l := range outLines {
        if strings.Contains(l, q) {
            res = append(res, l)
        }
    }
	clear()
	for _, l := range res {
		w.Write("body", []byte(l + "\n"))
	}
}

func doFuzzySearch(q string) {
	if debug {
		fmt.Fprintf(os.Stderr, "Search: %s\n", q)
		fmt.Fprintf(os.Stderr, "%v\n", outLines)
	}
	res := fuzzy.RankFind(q, outLines)
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Distance < res[j].Distance
	})
	clear()
	for _, l := range res {
		w.Write("body", []byte(l.Target + "\n"))
	}
}

func onLook(word string) {
	port, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		return
	}
	defer port.Close()
	msg := &plumb.Message{
		Src:  "afz",
		Dst:  "",
		Dir:  root,
		Type: "text",
		Data: []byte(word),
	}
	if err := msg.Send(port); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	return
}

func events() <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			estr := string(e.Text)
			switch e.C2 {
			case 'x':
				switch {
				case estr == "Del":
					w.Ctl("delete")
				case estr == "Reset":
					c <- "Reset"
				case (len(estr) >= 6 && estr[0:6] == "Search"):
					c <- string(e.Text)
				case (len(estr) >= 5 && estr[0:5] == "Fuzzy"):
					c <- string(e.Text)
				default:
					w.WriteEvent(e)
				}
			default:
				w.WriteEvent(e)
			}
		}
		w.CloseFiles()
		close(c)
	}()
	return c
}
