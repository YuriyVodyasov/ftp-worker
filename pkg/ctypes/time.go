package ctypes

import "time"

type TimeDuration time.Duration

func (t *TimeDuration) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))

	*t = TimeDuration(d)

	return err
}

func (t TimeDuration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(t).String()), nil
}

func (t TimeDuration) Duration() time.Duration {
	return time.Duration(t)
}
