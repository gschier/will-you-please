package main

import (
	"github.com/radovskyb/watcher"
	"log"
	"os"
	"path/filepath"
	"time"
)

func watchAndRepeat(dir string, cb func(e, p string)) {
	w := watcher.New()
	w.SetMaxEvents(1)

	// Only watch files, not directories
	w.AddFilterHook(func(info os.FileInfo, fullPath string) error {
		if info.IsDir() {
			return watcher.ErrSkip
		}

		return nil
	})

	// Watch test_folder recursively for changes.
	if err := w.AddRecursive(dir); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			select {
			case event := <-w.Event:
				wd, _ := os.Getwd()
				p, _ := filepath.Rel(wd, event.OldPath)
				cb(event.Op.String(), p)
			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	// Start the watching process - it'll check for changes every 100ms.
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}
