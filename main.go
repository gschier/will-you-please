package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const configFilePath = "./wyp.yaml"

type Script struct {
	Combine []string `yaml:"combine"`
	Dir     string   `yaml:"dir"`
	Env     []string `yaml:"env"`
	Help    string   `yaml:"help"`
	Hide    bool     `yaml:"hide"`
	Name    string   `yaml:"name"`
	Root    bool     `yaml:"root"`
	Run     string   `yaml:"run"`
	Shell   string   `yaml:"shell"`
	Watch   string   `yaml:"watch"`
}

func main() {
	ctx := context.Background()

	initViper()

	rootCmd := &cobra.Command{
		Use: "wyp",
	}

	// Start command will be overridden when creating run commands
	var cmdStart *cobra.Command

	// Detect a start command based on various project formats
	names, _ := ioutil.ReadDir(".")
	inspectors := viper.GetStringMap("inspectors")
	scripts := getScripts()
	for _, f := range names {
		if f.IsDir() {
			continue
		}

		// Don't overwrite if there is one already
		if _, ok := scripts["start"]; ok {
			continue
		}

		if (inspectors == nil || inspectors["npm"] != nil) && f.Name() == "package.json" {
			scripts["start"] = &Script{
				Name: "start",
				Run:  "npm start",
				Help: "npm start (detected)",
			}
		}

		if (inspectors == nil || inspectors["docker"] != nil) && f.Name() == "docker-compose.yml" {
			scripts["start"] = &Script{
				Name: "start",
				Run:  "docker-compose up",
				Help: "docker-compose up (detected)",
			}
		}

		if (inspectors == nil || inspectors["make"] != nil) && f.Name() == "Makefile" {
			scripts["start"] = &Script{
				Name: "start",
				Run:  "make",
				Help: "make (detected)",
			}
		}
	}

	cmdCombine := &cobra.Command{
		Use:   "combine [scripts...]",
		Short: "execute multiple scripts by name",
		Run: func(cmd *cobra.Command, args []string) {
			names := make([]string, 0)
			for i, name := range args {
				names = append(names, aurora.Colorize(name, getColor(i)).String())
			}

			name := fmt.Sprintf("[%s]", strings.Join(names, ", "))
			scripts[name] = &Script{
				Name:    name,
				Combine: args,
			}

			c, _ := newRunCmd(ctx, name, scripts)
			c.Run(cmd, nil)
		},
	}
	addWatchFlag(cmdCombine)

	cmdRun := &cobra.Command{
		Use:   "run [script]",
		Short: "run script by name",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				exitf("Script not found: %s", aurora.Bold(args[0]))
			}

			items := make([]string, 0)
			itemsMap := make(map[string]string)

			for name, script := range scripts {
				suffix := ""
				if script.Help != "" {
					suffix = fmt.Sprintf(": %s", script.Help)
				}

				markedUpItem := fmt.Sprintf("%s%s", aurora.Bold(name), suffix)
				itemsMap[markedUpItem] = name
				items = append(items, markedUpItem)
			}

			prompt := promptui.Select{
				Label:        aurora.Bold("Select a script to run"),
				Items:        items,
				Size:         20,
				HideHelp:     true,
				HideSelected: true,
			}

			_, result, err := prompt.Run()
			exitOnErr(err, "Failed to select script")

			scriptName := itemsMap[result]
			newCmd, _ := newRunCmd(ctx, scriptName, scripts)
			newCmd.Run(cmd, nil)
		},
	}
	addWatchFlag(cmdRun)

	for name := range scripts {
		cmd, script := newRunCmd(ctx, name, scripts)
		addWatchFlag(cmd)

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
	addWatchFlag(cmdStart)

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
		cmdCombine,
	)

	_ = rootCmd.Execute()
}

func addWatchFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolP("watch", "w", false, "restart when files change")
	cmd.PersistentFlags().StringP("watch-dir", "W", "", "restart when files change")
}

func newRunCmd(ctx context.Context, entryScriptName string, scripts map[string]*Script) (*cobra.Command, *Script) {
	entryScript := scripts[entryScriptName]

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

	// Fill in help for combine script if not there
	if entryScript.Combine != nil && entryScript.Help == "" {
		entryScript.Help = fmt.Sprintf("run in parallel %s", strings.Join(entryScript.Combine, ", "))
	}

	cmd := &cobra.Command{
		Use:    entryScriptName,
		Short:  entryScript.Help,
		Hidden: entryScript.Hide,
		Run: func(cmd *cobra.Command, args []string) {
			startTime := time.Now()
			fmt.Printf(
				"[wyp] Running %s at %s\n",
				aurora.Magenta(entryScriptName),
				aurora.Bold(startTime.Format(time.Kitchen)),
			)

			runScripts := make([]*Script, 0)
			for _, n := range entryScript.Combine {
				runScripts = append(runScripts, scripts[n])
			}

			rg := newRunGroup(ctx, runScripts)
			rg.Start()

			watchDir := defaultStr(entryScript.Watch, cmd.Flag("watch-dir").Value.String())
			watch, _ := cmd.Flags().GetBool("watch")
			if watchDir == "" && watch {
				watchDir = "."
			}

			if watchDir != "" {
				fmt.Printf("[wyp] Watching directory \"%s\"\n", watchDir)
				watchAndRepeat(watchDir, func(e, p string) {
					fmt.Printf("[wyp] Restarting %s (%s)\n", p, strings.ToLower(e))
					rg.Restart()
				})
			}

			err := rg.Wait()
			exitOnErr(err, "Error")

			fmt.Printf(
				"[wyp] Completed %s at %s in %s\n",
				entryScriptName,
				aurora.Bold(time.Now().Format(time.Kitchen)),
				aurora.Bold(ago(startTime)),
			)
		},
	}

	return cmd, entryScript
}

type prefixedWriter struct {
	w      io.Writer
	prefix string
	wrote  int
}

func newPrefixedWriter(w io.Writer, name string, color aurora.Color) *prefixedWriter {
	prefix := ""

	if name != "" {
		prefixStr := fmt.Sprintf("[%s] ", name)
		prefix = aurora.Colorize(prefixStr, color).String()
	}

	return &prefixedWriter{
		w:      w,
		prefix: prefix,
	}
}

func (p2 prefixedWriter) DidWrite() bool {
	return p2.wrote > 0
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

	n := len(p)
	p2.wrote += n

	return n, nil
}

func getScripts() map[string]*Script {
	scripts := make(map[string]*Script)
	_ = viper.UnmarshalKey("scripts", &scripts)
	for name, s := range scripts {
		if s.Name == "" {
			s.Name = name
		}
	}
	return scripts
}

func exitOnErr(err error, v ...interface{}) {
	if err == nil {
		return
	}

	exit(append(v, ": ", err.Error())...)
}

func exitOnTrue(ok bool, v ...interface{}) {
	if ok {
		return
	}

	exit(v...)
}

func exit(v ...interface{}) {
	exitf("%s", fmt.Sprint(v...))
}

func exitf(tmp string, v ...interface{}) {
	fmt.Printf("[wyp] "+tmp, v...)
	fmt.Print("\n\n")
	os.Exit(0)
}

func initViper() {
	viper.SetConfigName("wyp")
	viper.AddConfigPath(".")
	viper.SetDefault("Inspectors", map[string]interface{}{
		"npm":    true,
		"docker": true,
		"make":   true,
	})
	viper.SetDefault("Scripts", []interface{}{})

	err := viper.ReadInConfig()
	_, configNotFound := err.(viper.ConfigFileNotFoundError)
	if err != nil && configNotFound {
		// That's okay
	} else {
		exitOnErr(err, "Failed to read config file")
	}
}

func buildExecCmd(ctx context.Context, script *Script, name string, maxNameLength int, index int, usePrefix bool) *exec.Cmd {
	if runtime.GOOS == "windows" {
		exit("Windows is not currently supported")
	}

	prefix := name
	// if usePrefix {
	// 	prefix = name
	// }

	color := getColor(index)
	shell := defaultStr(script.Shell, os.Getenv("SHELL"), "bash")

	c := exec.CommandContext(ctx, shell, "-c", script.Run)
	c.Env = append(os.Environ(), script.Env...)
	c.Dir = script.Dir
	c.Stdin = os.Stdin
	c.Stdout = newPrefixedWriter(os.Stdout, prefix, color)
	c.Stderr = newPrefixedWriter(os.Stderr, prefix, color)
	return c
}
