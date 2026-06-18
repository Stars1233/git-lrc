package reviewquery

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

// RunQuery is the default action for `lrc query`. It either saves an alias
// (--add/--name) or runs a saved alias / raw SQL and prints a table or JSON.
func RunQuery(c *cli.Context) error {
	if add := strings.TrimSpace(c.String("add")); add != "" {
		name := strings.TrimSpace(c.String("name"))
		if name == "" {
			return fmt.Errorf("--add requires --name")
		}
		if err := AddAlias(name, add); err != nil {
			return err
		}
		fmt.Printf("Saved alias %q.\n", name)
		return nil
	}

	// urfave/cli stops parsing flags at the first positional arg, so support a
	// trailing --json too (e.g. `lrc query stats --json`).
	jsonOut := c.Bool("json")
	positionals := make([]string, 0, c.NArg())
	for _, a := range c.Args().Slice() {
		switch a {
		case "--json", "-json", "-j":
			jsonOut = true
		default:
			positionals = append(positionals, a)
		}
	}

	arg := "stats" // default alias
	if len(positionals) > 0 && strings.TrimSpace(positionals[0]) != "" {
		arg = strings.TrimSpace(positionals[0])
	}

	sqlText, found, err := ResolveAlias(arg)
	if err != nil {
		return err
	}
	if !found {
		// Not a known alias — treat the positional args as raw SQL.
		sqlText = strings.Join(positionals, " ")
	}

	res, err := Run(Filter{}, sqlText)
	if err != nil {
		return err
	}

	if jsonOut {
		out, err := FormatJSON(res)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}
	fmt.Print(FormatTable(res))
	return nil
}

// RunQueryList prints every alias and its source.
func RunQueryList(c *cli.Context) error {
	aliases, err := ListAliases()
	if err != nil {
		return err
	}
	if len(aliases) == 0 {
		fmt.Println("(no aliases)")
		return nil
	}
	for _, a := range aliases {
		fmt.Printf("%-18s [%s]\n", a.Name, a.Source)
	}
	return nil
}

// RunQueryView prints the SQL behind a named alias.
func RunQueryView(c *cli.Context) error {
	name := strings.TrimSpace(c.Args().First())
	if name == "" {
		return fmt.Errorf("usage: lrc query view <name>")
	}
	sqlText, found, err := ResolveAlias(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no alias named %q", name)
	}
	fmt.Println(sqlText)
	return nil
}

// RunQueryDelete removes a user-defined alias.
func RunQueryDelete(c *cli.Context) error {
	name := strings.TrimSpace(c.Args().First())
	if name == "" {
		return fmt.Errorf("usage: lrc query delete <name>")
	}
	if err := DeleteAlias(name); err != nil {
		return err
	}
	fmt.Printf("Deleted alias %q.\n", name)
	return nil
}
