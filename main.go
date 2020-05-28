package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const configFilePath = "./wyp.yaml"

type Script struct {
	Run     string            `yaml:"run"`
	Watch   string            `yaml:"watch"`
	Help    string            `yaml:"help"`
	Env     map[string]string `yaml:"env"`
	Combine []string          `yaml:"combine"`
	Dir     string            `yaml:"dir"`
	Hide    bool              `yaml:"hide"`
	Root    bool              `yaml:"root"`
}

func main() {
	initViper()

	rootCmd := &cobra.Command{
		Use: "wyp",
	}

	// Start command will be overridden when creating run commands
	var cmdStart *cobra.Command

	// Detect a start command based on various project formats
	names, _ := ioutil.ReadDir(".")
	inspectors := viper.GetStringMap("inspectors")
	for _, f := range names {
		if f.IsDir() {
			continue
		}

		if (inspectors == nil || inspectors["npm"] != nil) && f.Name() == "package.json" {
			cmdStart, _ = newRunCmd("start", map[string]Script{"start": {
				Run:  "npm start",
				Help: "[npm] start",
			}})
		}
	}

	cmdRun := &cobra.Command{
		Use:   "run [script]",
		Short: "execute script by name",
	}

	scripts := getScripts()
	for name := range scripts {
		cmd, script := newRunCmd(name, scripts)

		// Don't add start here. It will be
		if name == "start" {
			cmdStart = cmd
		}

		cmdRun.AddCommand(cmd)

		if script.Root {
			rootCmd.AddCommand(cmd)
		}
	}

	// Add default start command if we didn't find one with that name
	if cmdStart == nil {
		cmdStart = &cobra.Command{
			Use:   "start",
			Short: "shortcut to run the start script",
			Args:  cobra.MinimumNArgs(0),
			Run: func(cmd *cobra.Command, args []string) {
				exit("No start script defined")
			},
		}
	}

	cmdKill := &cobra.Command{
		Use:   "kill [pid]",
		Short: "kill a running process",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := exec.CommandContext(context.Background(), "kill", args[0])
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			fmt.Println("[wyp] Killing process", args[0])
			_ = c.Run()
		},
	}

	cmdInit := &cobra.Command{
		Use:   "init",
		Short: "create a new config file",
		Run: func(cmd *cobra.Command, args []string) {
			if fileExists(configFilePath) {
				exit("Directory already configured")
			}

			err := ioutil.WriteFile(configFilePath, []byte(strings.Join([]string{
				"scripts:",
				"  start:",
				"    combine: [ greet, sleep ]",
				"  greet:",
				"    help: say a greeting",
				"    run: echo Hello World!",
				"  sleep:",
				"    help: catch some z's",
				"    run: while true; do echo \"zzz\"; sleep 1; done",
			}, "\n")), 0644)
			exitOnErr(err, "Failed to write wyp.yaml")
			fmt.Println("[wyp] Generated scripts file at ./wyp.yaml")
		},
	}

	rootCmd.AddCommand(
		cmdInit,
		cmdStart,
		cmdRun,
		cmdKill,
	)

	_ = rootCmd.Execute()
}

func newRunCmd(entryScriptName string, scripts map[string]Script) (*cobra.Command, *Script) {
	entryScript := scripts[entryScriptName]
	overrideDir := ""
	prefixLogs := true

	// If not combine, add Run script to combine so we only
	// have to deal with that
	if entryScript.Combine == nil {
		entryScript.Combine = []string{entryScriptName}
		prefixLogs = false
	}

	maxNameLength := 0
	for _, n := range entryScript.Combine {
		if len(n) > maxNameLength {
			maxNameLength = len(n)
		}
	}

	// Fill in help for combine script if not there
	if entryScript.Combine != nil && entryScript.Help == "" {
		entryScript.Help = fmt.Sprintf("run in parallel %s", strings.Join(entryScript.Combine, ", "))
	}

	cmd := &cobra.Command{
		Use:    entryScriptName,
		Short:  entryScript.Help,
		Hidden: entryScript.Hide,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("[wyp] Running scripts.%s\n", entryScriptName)

			wg := sync.WaitGroup{}
			for i := 0; i < len(entryScript.Combine); i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, name string, index int) {
					defer wg.Done()

					script, ok := scripts[name]
					if !ok {
						fmt.Println("script not found for", name)
						os.Exit(1)
					}

					c := exec.CommandContext(context.Background(), "/bin/bash", "-c", script.Run)
					c.Dir = defaultStr(overrideDir, script.Dir)
					c.Stdin = os.Stdin
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr

					if prefixLogs {
						color := getColor(index)
						c.Stdout = newPrefixedWriter(os.Stdout, name, maxNameLength, color)
						c.Stderr = newPrefixedWriter(os.Stderr, name, maxNameLength, color)
					}

					err := c.Run()
					if err != nil {
						fmt.Println("[wyp] Failed to run", err)
						os.Exit(1)
					}
				}(&wg, entryScript.Combine[i], i)
			}

			wg.Wait()
		},
	}
	cmd.Flags().StringVar(&overrideDir, "dir", "", "directory to run from")
	return cmd, &entryScript
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

func defaultStr(str ...string) string {
	for _, s := range str {
		if s != "" {
			return s
		}
	}

	return ""
}

func getScripts() map[string]Script {
	scripts := make(map[string]Script)
	_ = viper.UnmarshalKey("scripts", &scripts)
	return scripts
}

func exitOnErr(err error, v ...interface{}) {
	if err == nil {
		return
	}

	exit(append(v, ": ", err.Error())...)
}

func exit(v ...interface{}) {
	exitf("%s", fmt.Sprint(v...))
}

func exitf(tmp string, v ...interface{}) {
	fmt.Printf("[wyp] "+tmp, v...)
	fmt.Print("\n")
	os.Exit(0)
}

func initViper() {
	viper.SetConfigName("wyp")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetDefault("Inspectors", map[string]interface{}{"npm": true})
	viper.SetDefault("Scripts", []interface{}{})

	err := viper.ReadInConfig()
	_, configNotFound := err.(viper.ConfigFileNotFoundError)
	if err != nil && configNotFound {
		// That's okay
	} else {
		exitOnErr(err, "Failed to read config file")
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return true
}

func debug(v interface{}) {
	b, _ := json.MarshalIndent(&v, "", "  ")
	fmt.Printf("\n[DEBUG] %s\n\n", b)
}
