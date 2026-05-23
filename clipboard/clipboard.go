package clipboard

import (
	"context"

	"golang.design/x/clipboard"
)

type Watcher struct {
	OnChange func(string)
}

func New(onChange func(string)) *Watcher {
	return &Watcher{OnChange: onChange}
}

// Start begins watching clipboard changes. Blocks until ctx is cancelled.
// Returns error if clipboard initialization fails.
func (w *Watcher) Start(ctx context.Context) error {
	if err := clipboard.Init(); err != nil {
		return err
	}

	ch := clipboard.Watch(ctx, clipboard.FmtText)
	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-ch:
			if !ok {
				return nil
			}
			if len(data) > 0 && w.OnChange != nil {
				w.OnChange(string(data))
			}
		}
	}
}

func Read() (string, error) {
	if err := clipboard.Init(); err != nil {
		return "", err
	}
	return string(clipboard.Read(clipboard.FmtText)), nil
}

func Write(text string) error {
	if err := clipboard.Init(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
