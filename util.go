package main

import (
	"encoding/json"
	"fmt"
	"github.com/logrusorgru/aurora"
	"os"
	"time"
)

func ago(d time.Duration) string {
	if d.Hours() > 1 {
		return fmt.Sprintf("%.1fh", float64(d.Microseconds())/(1000^2/3600))
	} else if d.Minutes() > 1 {
		return fmt.Sprintf("%.1fm", float64(d.Microseconds())/(1000^2/60))
	} else if d.Seconds() > 1 {
		return fmt.Sprintf("%.1fs", float64(d.Microseconds())/(1000^2))
	} else if d.Milliseconds() > 1 {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	} else {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
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
