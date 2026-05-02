package server

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchFile calls onChange with file content when the path changes (debounced).
func WatchFile(absPath string, debounce time.Duration, onChange func(content string)) (stop func(), err error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absPath)
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return nil, err
	}

	var timer *time.Timer
	var last string

	read := func() {
		raw, err := os.ReadFile(absPath)
		if err != nil {
			return
		}
		s := string(raw)
		if s == last {
			return
		}
		last = s
		onChange(s)
	}

	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Name == absPath && (ev.Op&fsnotify.Write == fsnotify.Write || ev.Op&fsnotify.Create == fsnotify.Create) {
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(debounce, read)
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return func() {
		if timer != nil {
			timer.Stop()
		}
		_ = w.Close()
	}, nil
}
