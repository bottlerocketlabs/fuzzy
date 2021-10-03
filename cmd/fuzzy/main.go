package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode"

	"github.com/bottlerocketlabs/fuzzy"
	"github.com/bottlerocketlabs/fuzzy/algo/smithwaterman"
)

// Env is abstracted environment
type Env struct {
	m map[string]string
}

// Get an environment variable by key, or blank string if missing
func (e *Env) Get(key string) string {
	value, ok := e.m[key]
	if !ok {
		return ""
	}
	return value
}

// NewEnv creates a new env from = separated string slice (eg: os.Environ())
func NewEnv(environ []string) Env {
	e := make(map[string]string)
	for _, env := range environ {
		parts := strings.SplitN(env, "=", 2)
		e[parts[0]] = parts[1]
	}
	return Env{m: e}
}

func main() {
	err := Run(os.Args, NewEnv(os.Environ()), os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
}

func HasUpper(str string) bool {
	for _, r := range str {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func NewSmithWaterman(caseSensitive bool) *smithwaterman.SmithWatermanGotoh {
	return &smithwaterman.SmithWatermanGotoh{
		CaseSensitive: caseSensitive,
		GapPenalty:    -2,
		Substitution: smithwaterman.MatchMismatch{
			Match:    1,
			Mismatch: -2,
		},
	}
}

// Run is the main thread but separated out so easier to test
func Run(args []string, env Env, stdin *os.File, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintf(stderr, `Usage:
	fuzzy [options] [query] - output selected line from stdin (fuzzy search)
`)
		flags.PrintDefaults()
	}
	verbose := flags.Bool("v", false, "verbose. print out scores with text")
	input := stdin
	err := flags.Parse(args[1:])
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}
	query := strings.Join(flags.Args(), " ")
	content := fuzzy.ReadNewContent(input)
	caseSensitive := HasUpper(query)
	content.SetTextScorer(NewSmithWaterman(caseSensitive))
	if *verbose {
		content.SetVerbose()
	}
	out, err := fuzzy.Find(query, content)
	fmt.Fprintln(stdout, out)
	return err
}
