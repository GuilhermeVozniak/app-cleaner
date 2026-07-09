package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// promptConfirm renders "{question} [y/N] " (or "[Y/n] " when defaultYes) to
// w, reads one line from in, and reports the user's answer. A bare Enter
// (empty line) takes defaultYes; any other input answers yes only when it
// starts with 'y' or 'Y'. Shared by clean, backups, and uninstall.
func promptConfirm(in io.Reader, w io.Writer, question string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	_, _ = fmt.Fprintf(w, "%s %s ", question, hint)
	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultYes
	}
	return strings.EqualFold(line[:1], "y")
}
