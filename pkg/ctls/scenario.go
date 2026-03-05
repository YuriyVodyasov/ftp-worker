package ctls

import "github.com/pkg/errors"

type Scenario string

const (
	ScenarioDomain Scenario = "domain"
	ScenarioIP     Scenario = "ip"
)

func (s Scenario) String() string {
	switch s {
	case ScenarioDomain:
		return "domain"
	case ScenarioIP:
		return "ip"
	default:
		return "unknown"
	}
}

func (s *Scenario) UnmarshalText(text []byte) error {
	str := string(text)

	switch str {
	case "domain":
		*s = ScenarioDomain
	case "ip":
		*s = ScenarioIP
	default:
		return errors.Errorf("invalid scenario: %s", str)
	}

	return nil
}

func (s Scenario) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}
