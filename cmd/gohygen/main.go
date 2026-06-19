// Command gohygen is a hygen-compatible code generator CLI backed by the
// gohygen engine. Usage:
//
//	gohygen <generator> <action> [--name NAME] [--key value ...] [--dry]
//
// Templates are read from $HYGEN_TMPLS or ./_templates. Flags become template
// locals (so --name widget exposes `name`, `Name`, `names`, `Names`); --dry
// previews without writing.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/elioria/gohygen"
)

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gohygen <generator> <action> [--key value ...] [--dry]")
		os.Exit(2)
	}
	generator, action := args[0], args[1]

	locals := map[string]any{}
	dry := false
	rest := args[2:]
	var positionals []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--dry":
			dry = true
		case strings.HasPrefix(a, "--"):
			key := strings.TrimPrefix(a, "--")
			// support --key=value and --key value
			if eq := strings.IndexByte(key, '='); eq >= 0 {
				locals[key[:eq]] = key[eq+1:]
				continue
			}
			if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "--") {
				locals[key] = rest[i+1]
				i++
			} else {
				locals[key] = true // boolean flag
			}
		default:
			positionals = append(positionals, a)
		}
	}
	// First bare positional fills `name` (hygen convention) if not set via flag.
	if _, ok := locals["name"]; !ok && len(positionals) > 0 {
		locals["name"] = positionals[0]
	}

	eng := gohygen.NewEngine(gohygen.Config{
		Dry:    dry,
		Logger: func(s string) { fmt.Println(s) },
	})

	res, err := eng.Run(generator, action, locals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if dry {
		fmt.Println("(dry run — no files written)")
	}
	_ = res
}
