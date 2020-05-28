package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
)

/*
scripts:
	start:
		run:
*/

type Config struct {
	Scripts map[string]struct {
		Run        string            `yaml:"run"`
		Background bool              `yaml:"background"`
		Watch      string            `yaml:"watch"`
		Env        map[string]string `yaml:"env"`
		Combine    []string          `yaml:"combine"`
		Dir        string            `yaml:"dir"`
	} `yaml:"scripts"`
}

func main() {
	f, err := os.Open("wyp.yaml")
	if err != nil {
		fmt.Println(ioutil.ReadDir("."))
		panic(err)
	}
	defer f.Close()

	var conf Config
	err = yaml.NewDecoder(f).Decode(&conf)
	if err != nil {
		panic(err)
	}

	cmdPrint := &cobra.Command{
		Use:     "print [string to print]",
		Short:   "Print anything to the screen",
		Example: "print Hello World!",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	cmdRun := &cobra.Command{
		Use:   "run [command]",
		Short: "run a command by name",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			entryScriptName := args[0]
			entryScript, ok := conf.Scripts[entryScriptName]
			if !ok {
				fmt.Println("script not found for", args[0])
				os.Exit(1)
			}

			// If not combine, add Run script to combine so we only
			// have to deal with that
			if entryScript.Combine == nil {
				entryScript.Combine = []string{entryScriptName}
			}

			maxNameLength := 0
			for _, n := range entryScript.Combine {
				if len(n) > maxNameLength {
					maxNameLength = len(n)
				}
			}

			wg := sync.WaitGroup{}
			for i := 0; i < len(entryScript.Combine); i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, name string, index int) {
					defer wg.Done()

					script, ok := conf.Scripts[name]
					if !ok {
						fmt.Println("script not found for", name)
						os.Exit(1)
					}

					c := exec.CommandContext(context.Background(), "/bin/bash", "-c", script.Run)
					c.Dir = script.Dir
					c.Stdin = os.Stdin
					color := getColor(index)
					c.Stdout = newPrefixedWriter(os.Stdout, name, maxNameLength, color)
					c.Stderr = newPrefixedWriter(os.Stderr, name, maxNameLength, color)

					err := c.Start()
					if err != nil {
						fmt.Println("[wyp] Failed to run", err)
						os.Exit(1)
					}

					if !entryScript.Background {
						_ = c.Wait()
					}
				}(&wg, entryScript.Combine[i], i)
			}

			wg.Wait()
		},
	}

	cmdStart := &cobra.Command{
		Use:   "start",
		Short: "shortcut to run the start script",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdRun.Run(cmd, append([]string{"start"}, args...))
		},
	}

	cmdKill := &cobra.Command{
		Use:   "kill [pid]",
		Short: "kill it",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := exec.CommandContext(context.Background(), "kill", args[0])
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			fmt.Println("[wyp] Killing process", args[0])
			_ = c.Run()
		},
	}

	rootCmd := &cobra.Command{Use: "wyp"}
	rootCmd.AddCommand(cmdPrint, cmdRun, cmdStart, cmdKill)

	_ = rootCmd.Execute()
}

type prefixedWriter struct {
	w      io.Writer
	prefix string
}

func newPrefixedWriter(w io.Writer, name string, length int, color aurora.Color) *prefixedWriter {
	padBy := length - len(name)
	padding := ""

	for i := 0; i < padBy; i++ {
		padding += " "
	}

	prefix := fmt.Sprintf("[%s] %s", name, padding)
	return &prefixedWriter{w: w, prefix: aurora.Colorize(prefix, color).String()}
}

func (p2 prefixedWriter) Write(p []byte) (int, error) {
	// Split on newlines so we can prefix each one
	lines := bytes.Split(p, []byte{'\n'})
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		line := append(l, '\n')
		_, _ = p2.w.Write(append([]byte(p2.prefix), line...))
	}
	return len(p), nil
}

func getColor(i int) aurora.Color {
	colors := []aurora.Color{
		aurora.MagentaFg,
		aurora.BlueFg,
		aurora.YellowFg,
		aurora.CyanFg,
		aurora.GreenFg,
		aurora.RedFg,
	}

	return colors[i%len(colors)]
}
