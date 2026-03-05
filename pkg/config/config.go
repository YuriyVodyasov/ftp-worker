package config

import (
	"io/fs"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func Get[T any](configFile string) (*T, error) {
	var conf T

	file, err := os.ReadFile(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	err = yaml.Unmarshal(file, &conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config file")
	}

	return &conf, nil
}

func Set[T any](configFile string, conf *T) error {
	toWrite, err := yaml.Marshal(conf)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config file")
	}

	err = os.WriteFile(configFile, toWrite, fs.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to read config file")
	}

	return nil
}
