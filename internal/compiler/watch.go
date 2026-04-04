package compiler

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/xoai/sage-wiki/internal/config"
	"github.com/xoai/sage-wiki/internal/log"
)

// Watch monitors source directories for changes and triggers compilation.
func Watch(projectDir string, debounceSeconds int) error {
	if debounceSeconds <= 0 {
		debounceSeconds = 2
	}

	cfgPath := filepath.Join(projectDir, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add source directories recursively
	sourcePaths := cfg.ResolveSources(projectDir)
	for _, sp := range sourcePaths {
		if err := addRecursive(watcher, sp); err != nil {
			log.Warn("failed to watch directory", "path", sp, "error", err)
		}
	}

	log.Info("watching for changes", "sources", sourcePaths, "debounce", debounceSeconds)

	debounce := time.Duration(debounceSeconds) * time.Second
	var timer *time.Timer
	var compileMu sync.Mutex
	var compiling atomic.Bool
	var lastTrigger string

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			log.Debug("file change detected", "path", event.Name, "op", event.Op)
			lastTrigger = event.Name

			// Reset debounce timer
			if timer != nil {
				timer.Stop()
			}
			trigger := lastTrigger
			timer = time.AfterFunc(debounce, func() {
				// Prevent overlapping compiles
				if compiling.Load() {
					log.Info("compile already in progress, skipping", "trigger", trigger)
					return
				}
				compileMu.Lock()
				defer compileMu.Unlock()
				compiling.Store(true)
				defer compiling.Store(false)

				log.Info("compiling after change", "trigger", trigger)
				result, err := Compile(projectDir, CompileOpts{})
				if err != nil {
					log.Error("compile failed", "error", err)
				} else {
					log.Info("compile complete",
						"summarized", result.Summarized,
						"concepts", result.ConceptsExtracted,
						"articles", result.ArticlesWritten,
					)
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error("watcher error", "error", err)
		}
	}
}

// addRecursive adds a directory and all subdirectories to the watcher.
func addRecursive(watcher *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
}
