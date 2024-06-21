package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	LOG_MODE_PRETTY = "pretty"
	LOG_MODE_JSON   = "json"

	LOG_LEVEL_DEBUG    = "debug"
	LOG_LEVEL_INFO     = "info"
	LOG_LEVEL_WARN     = "warn"
	LOG_LEVEL_ERROR    = "error"
	LOG_LEVEL_FATAL    = "fatal"
	LOG_LEVEL_PANIC    = "panic"
	LOG_LEVEL_DISABLED = "disabled"
)

type Config struct {
	Mode  string // pretty, json
	Level string // debug, info, warn, error, fatal, panic, disabled
}

func Init(cfg Config) error {
	zerolog.CallerMarshalFunc = formatCaller
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.With().Caller().Logger()

	switch cfg.Mode {
	case LOG_MODE_PRETTY:
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	case LOG_MODE_JSON:
		log.Logger = log.Output(os.Stderr)
	default:
		return fmt.Errorf("invalid log mode: %s", cfg.Mode)
	}

	switch cfg.Level {
	case LOG_LEVEL_DEBUG:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LOG_LEVEL_INFO:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LOG_LEVEL_WARN:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LOG_LEVEL_ERROR:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case LOG_LEVEL_FATAL:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case LOG_LEVEL_PANIC:
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case LOG_LEVEL_DISABLED:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	return nil
}

func Disable() {
	log.Logger = log.Output(zerolog.Nop())
}

type Logger struct {
	zerolog.Logger
}

func New(pkg, funcName string) *Logger {
	log := log.Logger.With().Str("pkg", pkg).Str("func", funcName).Logger()
	return &Logger{log}
}

func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info().Msgf(msg, args...)
}

func (l *Logger) Error(msg string, err error) {
	l.Logger.Error().Stack().Err(err).Msg(msg) // need to print stack trace
}

func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug().Msgf(msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Fatal().Msgf(msg, args...)
}

func (l *Logger) Panic(msg string, args ...any) {
	l.Logger.Panic().Msgf(msg, args...)
}

func formatCaller(pc uintptr, file string, line int) string {
	return filepath.Base(file) + ":" + strconv.Itoa(line)
}