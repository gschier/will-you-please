package main

import (
	"context"
	"fmt"
	"github.com/gschier/wyp/internal"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const configFilePath = "./wyp.yaml"

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
			scripts["start"] = &script{
				Name: "start",
				Run:  "npm start",
				Help: "npm start (detected)",
			}
		}

		if (inspectors == nil || inspectors["docker"] != nil) && f.Name() == "docker-compose.yml" {
			scripts["start"] = &script{
				Name: "start",
				Run:  "docker-compose up",
				Help: "docker-compose up (detected)",
			}
		}

		if (inspectors == nil || inspectors["make"] != nil) && f.Name() == "Makefile" {
			scripts["start"] = &script{
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
				names = append(names, aurora.Colorize(name, internal.GetColor(i)).String())
			}

			name := fmt.Sprintf("[%s]", strings.Join(names, ", "))
			scripts[name] = &script{
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
			if internal.FileExists(configFilePath) {
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
	if cmd.Flag("watch") != nil {
		return
	}

	cmd.Flags().BoolP("watch", "w", false, "restart when files change")
	cmd.Flags().StringP("watch-dir", "W", "", "restart when files change")
}

func newRunCmd(ctx context.Context, entryScriptName string, scripts map[string]*script) (*cobra.Command, *script) {
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

			runScripts := make([]internal.Script, 0)
			for _, n := range entryScript.Combine {
				runScripts = append(runScripts, newScriptWrapper(scripts[n]))
			}

			rg := internal.NewRunGroup(ctx, runScripts)
			rg.Start()

			watchDir := internal.DefaultStr(entryScript.Watch, cmd.Flag("watch-dir").Value.String())
			watch, _ := cmd.Flags().GetBool("watch")
			if watchDir == "" && watch {
				watchDir = "."
			}

			if watchDir != "" {
				fmt.Printf("[wyp] Watching directory \"%s\"\n", watchDir)
				internal.WatchAndRepeat(watchDir, func(e, p string) {
					fmt.Printf("[wyp] Restarting from change to %s (%s)\n", p, strings.ToLower(e))
					rg.Restart()
				})
			}

			err := rg.Wait()
			exitOnErr(err, "Error")

			fmt.Printf(
				"[wyp] Completed %s at %s in %s\n",
				entryScriptName,
				aurora.Bold(time.Now().Format(time.Kitchen)),
				aurora.Bold(internal.Ago(startTime)),
			)
		},
	}
	addWatchFlag(cmd)

	return cmd, entryScript
}

func getScripts() map[string]*script {
	scripts := make(map[string]*script)
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

type script struct {
	Combine []string `mapstructure:"combine"`
	Dir     string   `mapstructure:"dir"`
	Env     []string `mapstructure:"env"`
	Help    string   `mapstructure:"help"`
	Hide    bool     `mapstructure:"hide"`
	Name    string   `mapstructure:"name"`
	Root    bool     `mapstructure:"root"`
	Run     string   `mapstructure:"run"`
	Shell   string   `mapstructure:"shell"`
	Watch   string   `mapstructure:"watch"`
	Prefix  string   `mapstructure:"prefix"`
}

type scriptWrapper struct {
	s *script
}

func newScriptWrapper(s *script) *scriptWrapper {
	return &scriptWrapper{s: s}
}

func (w *scriptWrapper) Dir() string {
	return w.s.Dir
}

func (w *scriptWrapper) Env() []string {
	return w.s.Env
}

func (w *scriptWrapper) Name() string {
	return w.s.Name
}

func (w *scriptWrapper) Run() string {
	return w.s.Run
}

func (w *scriptWrapper) Shell() string {
	return w.s.Shell
}

func (w *scriptWrapper) Prefix() string {
	return w.s.Prefix
}
