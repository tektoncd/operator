package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mgutz/ansi"
)

var (
	magenta    = ansi.ColorFunc("magenta")
	cyan       = ansi.ColorFunc("cyan")
	red        = ansi.ColorFunc("red")
	redBold    = ansi.ColorFunc("red+b")
	yellow     = ansi.ColorFunc("yellow")
	blue       = ansi.ColorFunc("blue")
	blueBold   = ansi.ColorFunc("blue+b")
	green      = ansi.ColorFunc("green")
	greenBold  = ansi.ColorFunc("green+b")
	gray       = ansi.ColorFunc("black+i")
	bold       = ansi.ColorFunc("default+b")
	dimmed     = ansi.ColorFunc("246")
	underline  = ansi.ColorFunc("default+u")
	cyanBold   = ansi.ColorFunc("cyan+b")
	orangeBold = ansi.ColorFunc("208")

	gray256 = func(t string) string {
		return fmt.Sprintf("\x1b[%d;5;%dm%s\x1b[m", 38, 242, t)
	}
	hyperLink = func(title, href string) string {
		return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", href, title)
	}
)

func EnvColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0"
}

func EnvColorForced() bool {
	return os.Getenv("CLICOLOR_FORCE") != "" && os.Getenv("CLICOLOR_FORCE") != "0"
}

func Is256ColorSupported() bool {
	term := os.Getenv("TERM")
	colorterm := os.Getenv("COLORTERM")

	return strings.Contains(term, "256") ||
		strings.Contains(term, "24bit") ||
		strings.Contains(term, "truecolor") ||
		strings.Contains(colorterm, "256") ||
		strings.Contains(colorterm, "24bit") ||
		strings.Contains(colorterm, "truecolor")
}

func NewColorScheme(enabled, is256enabled bool) *ColorScheme {
	return &ColorScheme{
		enabled:      enabled,
		is256enabled: is256enabled,
	}
}

type ColorScheme struct {
	enabled      bool
	is256enabled bool
}

func (c *ColorScheme) ColorStatus(status string) string {
	switch strings.ToLower(status) {
	case "succeeded":
		return c.Green(status)
	case "failed":
		return c.Red(status)
	case "pipelineruntimeout":
		return c.Yellow("Timeout")
	case "norun":
		return c.Dimmed(status)
	case "running":
		return c.Blue(status)
	}
	return status
}

func (c *ColorScheme) Orange(t string) string {
	if !c.enabled {
		return t
	}
	return orangeBold(t)
}

func (c *ColorScheme) Bold(t string) string {
	if !c.enabled {
		return t
	}
	return bold(t)
}

func (c *ColorScheme) Dimmed(t string) string {
	if !c.enabled {
		return t
	}
	return dimmed(t)
}

func (c *ColorScheme) Boldf(t string, args ...interface{}) string {
	return c.Bold(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Red(t string) string {
	if !c.enabled {
		return t
	}
	return red(t)
}

func (c *ColorScheme) RedBold(t string) string {
	if !c.enabled {
		return t
	}
	return redBold(t)
}

func (c *ColorScheme) Bullet() string {
	if !c.enabled {
		return ""
	}

	return "∙ "
}

func (c *ColorScheme) BulletSpace() string {
	if !c.enabled {
		return ""
	}

	return "  "
}

func (c *ColorScheme) Redf(t string, args ...interface{}) string {
	return c.Red(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Yellow(t string) string {
	if !c.enabled {
		return t
	}
	return yellow(t)
}

func (c *ColorScheme) Yellowf(t string, args ...interface{}) string {
	return c.Yellow(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Green(t string) string {
	if !c.enabled {
		return t
	}
	return green(t)
}

func (c *ColorScheme) Underline(t string) string {
	if !c.enabled {
		return t
	}
	return underline(t)
}

func (c *ColorScheme) Greenf(t string, args ...interface{}) string {
	return c.Green(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Gray(t string) string {
	if !c.enabled {
		return t
	}
	if c.is256enabled {
		return gray256(t)
	}
	return gray(t)
}

func (c *ColorScheme) Grayf(t string, args ...interface{}) string {
	return c.Gray(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Magenta(t string) string {
	if !c.enabled {
		return t
	}
	return magenta(t)
}

func (c *ColorScheme) Magentaf(t string, args ...interface{}) string {
	return c.Magenta(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) Cyan(t string) string {
	if !c.enabled {
		return t
	}
	return cyan(t)
}

func (c *ColorScheme) Cyanf(t string, args ...interface{}) string {
	return c.Cyan(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) CyanBold(t string) string {
	if !c.enabled {
		return t
	}
	return cyanBold(t)
}

func (c *ColorScheme) Blue(t string) string {
	if !c.enabled {
		return t
	}
	return blue(t)
}

func (c *ColorScheme) BlueBold(t string) string {
	if !c.enabled {
		return t
	}
	return blueBold(t)
}

func (c *ColorScheme) Bluef(t string, args ...interface{}) string {
	return c.Blue(fmt.Sprintf(t, args...))
}

func (c *ColorScheme) SuccessIcon() string {
	return c.SuccessIconWithColor(c.Green)
}

func (c *ColorScheme) InfoIcon() string {
	return c.BlueBold("ℹ")
}

func (c *ColorScheme) SuccessIconWithColor(colo func(string) string) string {
	return colo("✓")
}

func (c *ColorScheme) WarningIcon() string {
	return c.Yellow("!")
}

func (c *ColorScheme) FailureIcon() string {
	return c.FailureIconWithColor(c.Red)
}

func (c *ColorScheme) FailureIconWithColor(colo func(string) string) string {
	return colo("X")
}

func (c *ColorScheme) ColorFromString(s string) func(string) string {
	s = strings.ToLower(s)
	var fn func(string) string
	switch s {
	case "bold":
		fn = c.Bold
	case "red":
		fn = c.Red
	case "yellow":
		fn = c.Yellow
	case "green":
		fn = c.Green
	case "gray":
		fn = c.Gray
	case "magenta":
		fn = c.Magenta
	case "cyan":
		fn = c.Cyan
	case "blue":
		fn = c.Blue
	default:
		fn = func(s string) string {
			return s
		}
	}

	return fn
}

func (c *ColorScheme) GreenBold(s string) string {
	if !c.enabled {
		return s
	}
	return greenBold(s)
}

func (c *ColorScheme) HyperLink(title, href string) string {
	if !c.enabled {
		return title
	}
	return hyperLink(title, href)
}
