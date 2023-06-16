package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"

	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/mgutz/ansi"
)

type IOStreams struct {
	In     io.ReadCloser
	Out    io.Writer
	ErrOut io.Writer

	colorEnabled             bool
	progressIndicatorEnabled bool
	stdoutTTYOverride        bool
	stderrTTYOverride        bool
	stderrIsTTY              bool
	stdoutIsTTY              bool
	is256enabled             bool
}

func (s *IOStreams) ColorScheme() *ColorScheme {
	return NewColorScheme(s.ColorEnabled(), s.ColorSupport256())
}

func (s *IOStreams) ColorEnabled() bool {
	return s.colorEnabled
}

func (s *IOStreams) SetColorEnabled(colorEnabled bool) {
	s.colorEnabled = colorEnabled
	s.setSurveyColor()
	s.progressIndicatorEnabled = colorEnabled
}

func (s *IOStreams) setSurveyColor() {
	if !s.colorEnabled {
		surveyCore.DisableColor = true
	} else {
		// override survey's poor choice of color
		surveyCore.TemplateFuncsWithColor["color"] = func(style string) string {
			switch style {
			case "white":
				if s.ColorSupport256() {
					return fmt.Sprintf("\x1b[%d;5;%dm", 38, 242)
				}
				return ansi.ColorCode("default")
			default:
				return ansi.ColorCode(style)
			}
		}
	}
}

func (s *IOStreams) ColorSupport256() bool {
	return s.is256enabled
}

func NewIOStreams() *IOStreams {
	stdoutIsTTY := isTerminal(os.Stdout)
	stderrIsTTY := isTerminal(os.Stderr)

	ios := &IOStreams{
		In:           os.Stdin,
		Out:          colorable.NewColorable(os.Stdout),
		ErrOut:       colorable.NewColorable(os.Stderr),
		colorEnabled: EnvColorForced() || (!EnvColorDisabled() && stdoutIsTTY),
		is256enabled: Is256ColorSupported(),
	}

	// the colours are not working on windows, let's disable it
	if runtime.GOOS == "windows" {
		ios.colorEnabled = false
	}

	if stdoutIsTTY && stderrIsTTY {
		ios.progressIndicatorEnabled = true
	}

	ios.setSurveyColor()

	// prevent duplicate isTerminal queries now that we know the answer
	ios.SetStdoutTTY(stdoutIsTTY)
	ios.SetStderrTTY(stderrIsTTY)
	return ios
}

func (s *IOStreams) SetStdoutTTY(isTTY bool) {
	s.stdoutTTYOverride = true
	s.stdoutIsTTY = isTTY
}

func (s *IOStreams) SetStderrTTY(isTTY bool) {
	s.stderrTTYOverride = true
	s.stderrIsTTY = isTTY
}

func (s *IOStreams) IsStdoutTTY() bool {
	if s.stdoutTTYOverride {
		return s.stdoutIsTTY
	}
	if stdout, ok := s.Out.(*os.File); ok {
		return isTerminal(stdout)
	}
	return false
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func IOTest() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:     io.NopCloser(in),
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}
