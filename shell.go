package gohygen

import (
	"os/exec"
	"strings"
)

// defaultShellRunner executes a command through /bin/sh -c, piping the rendered
// body to its standard input (matching hygen's execa shell behavior). The
// combined stdout+stderr is returned.
func defaultShellRunner(cmd, stdin string) (string, error) {
	c := exec.Command("/bin/sh", "-c", cmd)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	out, err := c.CombinedOutput()
	return string(out), err
}
