package main

import (
	"encoding/json"
	"fmt"
	"github.com/logrusorgru/aurora"
	"os"
	"time"
)

func ago(t time.Time) string {
	d := time.Now().Sub(t)

	if d.Hours() > 1 {
		return fmt.Sprintf("%.2fh", d.Minutes()/60)
	}

	if d.Minutes() > 1 {
		return fmt.Sprintf("%.2fm", d.Seconds()/60)
	}

	if d.Seconds() > 1 {
		return fmt.Sprintf("%.2fs", float32(d.Milliseconds())/1000)
	}

	if d.Milliseconds() > 1 {
		return fmt.Sprintf("%.2fms", float32(d.Microseconds())/1000)
	}

	return fmt.Sprintf("%dμs", d.Microseconds())
}

func debug(v interface{}) {
	b, _ := json.MarshalIndent(&v, "", "  ")
	fmt.Printf("\n[DEBUG] %s\n\n", b)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return true
}

func defaultStr(str ...string) string {
	for _, s := range str {
		if s != "" {
			return s
		}
	}

	return ""
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
