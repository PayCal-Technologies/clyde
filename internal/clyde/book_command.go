package clyde

import (
	"flag"
	"io"
	"strings"
	"time"
)

func cmdBook(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printBookHelp(out)
		return flag.ErrHelp
	}
	if len(args) == 0 {
		return errf("book requires subject")
	}
	plan, err := NewBookPlan(strings.Join(args, " "), time.Now())
	if err != nil {
		return err
	}
	printBookPlan(out, plan)
	return nil
}
