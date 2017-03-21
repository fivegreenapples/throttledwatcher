package throttledwatcher

import (
	"errors"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a particular file, delivering events to a channel
// after a specified period of calm.
type Watcher struct {
	C        chan struct{}
	stopChan chan struct{}
}

// NewWatcher establishes a new watch on file, and will deliver events
// to its channel after deadTime has elasped without any OS events
// seen on the file.
func NewWatcher(file string, deadTime time.Duration) (*Watcher, error) {
	fileToWatch, err := filepath.Abs(file)
	if err != nil {
		return nil, errors.New("Couldn't get absolute path of file. " + err.Error())
	}
	directoryToWatch := filepath.Dir(fileToWatch)

	w := Watcher{
		C:        make(chan struct{}),
		stopChan: make(chan struct{}),
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.New("Couldn't establish watcher. " + err.Error())
	}

	go func() {
		t := time.NewTimer(deadTime)
		timerRunning := true
		for {
			select {
			case event := <-watcher.Events:
				// Received an event. Check it's for our file.
				eventFile, evErr := filepath.Abs(event.Name)
				if evErr != nil || eventFile != fileToWatch {
					continue
				}
				// It's for our file so stop and restart the timer.
				if timerRunning {
					if !t.Stop() {
						// empty the timer chan if we failed to stop it
						<-t.C
					}
				}
				t.Reset(deadTime)
				timerRunning = true
			case watcherErr := <-watcher.Errors:
				log.Println("Throttled Watcher error:", watcherErr)
			case <-t.C:
				timerRunning = false
				w.C <- struct{}{}
			case <-w.stopChan:
				if timerRunning {
					t.Stop()
				}
				watcher.Close()
				return
			}
		}
	}()

	err = watcher.Add(directoryToWatch)
	if err != nil {
		w.stopChan <- struct{}{}
		return nil, errors.New("Couldn't watch directory. " + err.Error())
	}

	return &w, nil

}

// Stop closes the underlying watcher and prevents further events from
// being delivered.
func (w *Watcher) Stop() {
	w.stopChan <- struct{}{}
}
