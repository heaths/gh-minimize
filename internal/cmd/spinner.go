package cmd

import (
	"io"

	appterm "github.com/heaths/gh-minimize/internal/terminal"
)

type progressIndicator interface {
	Start()
	Stop()
}

var newProgressIndicator = func(output io.Writer) progressIndicator {
	return appterm.NewSpinner(output)
}

func startProgress(output io.Writer) progressIndicator {
	indicator := newProgressIndicator(output)
	if indicator != nil {
		indicator.Start()
	}

	return indicator
}

func stopProgress(indicator progressIndicator) {
	if indicator != nil {
		indicator.Stop()
	}
}
