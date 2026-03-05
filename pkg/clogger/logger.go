package clogger

import (
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level       zerolog.Level `yaml:"level"`
	Development bool          `yaml:"development"`
	EnableFile  bool          `yaml:"enableFile"`
	LogPath     string        `yaml:"logPath"`
	MaxSize     int           `yaml:"maxSize"`
	MaxBackups  int           `yaml:"maxBackups"`
	MaxAge      int           `yaml:"maxAge"`
	Compress    bool          `yaml:"compress"`
}

func New(cfg Config) Logger {
	zerolog.SetGlobalLevel(cfg.Level)

	var multi zerolog.LevelWriter

	if cfg.Development {
		multi = zerolog.MultiLevelWriter(
			zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: zerolog.TimeFormatUnix})
	} else {
		multi = zerolog.MultiLevelWriter(os.Stdout)
	}

	if cfg.EnableFile {
		hook := &lumberjack.Logger{
			Filename:   cfg.LogPath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		multi = zerolog.MultiLevelWriter(multi, hook)
	}

	logger := zerolog.New(multi).With().Timestamp().Logger()

	return Logger{logger}
}

type Logger struct {
	zerolog.Logger
}

func (l Logger) Errorf(message string, args ...any) {
	l.Logger.Error().Msgf(message, args...)
}

func (l Logger) Fatalf(message string, args ...any) {
	l.Logger.Fatal().Msgf(message, args...)
}

func (l Logger) Infof(message string, args ...any) {
	l.Logger.Info().Msgf(message, args...)
}
