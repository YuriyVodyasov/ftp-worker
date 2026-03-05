package options

import (
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
)

type Opts struct {
	ConfigFile string `long:"config" env:"CONFIG" default:"/config-app/config.yaml" description:"config yaml file"`
	SecretFile string `long:"secret" env:"SECRET" default:"/secret-app/secret.yaml" description:"secret yaml file"`
	Debug      bool   `long:"debug" env:"DEBUG" description:"debug mode"`
}

func Get() *Opts {
	if err := loadEnvironment(); err != nil {
		panic(err)
	}

	var opts Opts

	parser := flags.NewParser(&opts, flags.Default)

	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}

		os.Exit(1)
	}

	return &opts
}

func loadEnvironment() error {
	if os.Getenv("SECRET") == "" || os.Getenv("CONFIG") == "" {
		return godotenv.Load()
	}

	return nil
}
