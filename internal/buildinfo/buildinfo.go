package buildinfo

import "fmt"

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type Info struct {
	Version string
	Commit  string
	Date    string
}

func Current() Info {
	return Info{
		Version: fallback(version, "dev"),
		Commit:  commit,
		Date:    date,
	}
}

func (i Info) String() string {
	if i.Commit == "" && i.Date == "" {
		return i.Version
	}
	if i.Commit == "" {
		return fmt.Sprintf("%s (%s)", i.Version, i.Date)
	}
	if i.Date == "" {
		return fmt.Sprintf("%s (%s)", i.Version, i.Commit)
	}
	return fmt.Sprintf("%s (%s, %s)", i.Version, i.Commit, i.Date)
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
