package commonapp

import "fmt"

type Version struct {
	Name   string `json:"name"`
	Ver    string `json:"version"`
	Commit string `json:"commit"`
	Date   string `json:"build_date"`
	Dirty  string `json:"dirty"`
}

func (v *Version) String() string {
	return fmt.Sprintf("name: %s, version: %s, commit: %s, build date: %s, dirty: %s",
		v.Name,
		v.Ver,
		v.Commit,
		v.Date,
		v.Dirty)
}
