package cli

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// PacCliOpts is the struct that holds all the options for the CLI
// TODO: Pass this to a context.
type PacCliOpts struct {
	NoColoring    bool
	AllNameSpaces bool
	Namespace     string
	UseRealTime   bool
	AskOpts       survey.AskOpt
	NoHeaders     bool
}

func NewAskopts(opt *survey.AskOptions) error {
	opt.Stdio = terminal.Stdio{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}
	return nil
}

func NewCliOptions() *PacCliOpts {
	return &PacCliOpts{
		AskOpts: NewAskopts,
	}
}
