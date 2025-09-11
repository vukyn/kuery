package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

const (
	LOG_MODE_PRETTY = "pretty"
	LOG_MODE_JSON   = "json"

	LOG_LEVEL_DEBUG    = "debug"
	LOG_LEVEL_INFO     = "info"
	LOG_LEVEL_WARN     = "warn"
	LOG_LEVEL_ERROR    = "error"
	LOG_LEVEL_PANIC    = "panic"
	LOG_LEVEL_DISABLED = "disabled"
)

type Config struct {
	Mode  string // pretty, json
	Level string // debug, info, warn, error, panic, disabled
}

func Init(cfg Config) error {
	zerolog.CallerMarshalFunc = formatCaller // format caller (log.go:77)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

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
	case LOG_LEVEL_PANIC:
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case LOG_LEVEL_DISABLED:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	return nil
}

type LoggerOption func(*zerolog.Event) error

type SimpleLogger interface {
	Info(msg string)
	Infof(format string, v ...any)
	Error(msg string)
	Errorf(format string, v ...any)
	Debug(msg string)
	Debugf(msg string, args ...any)
	Warn(msg string)
	Warnf(msg string, args ...any)
	Fatal(msg string, err error)
	Fatalf(msg string, args ...any)
	Panic(msg string)
	Panicf(msg string, args ...any)
}

type Logger interface {
	SimpleLogger
	WithField(key string, value any) Logger
	WithPkg(pkg string) Logger
	WithFunc(funcName string) Logger
}

type logger struct {
	zerolog.Logger
	opts []LoggerOption
}

func New() Logger {
	log := log.With().
		Caller().
		CallerWithSkipFrameCount(3).
		Logger()
	return &logger{Logger: log}
}

func (l *logger) WithField(key string, value any) Logger {
	l.opts = append(l.opts, func(event *zerolog.Event) error {
		event.Any(key, value)
		return nil
	})
	return l
}

func (l *logger) WithPkg(pkg string) Logger {
	l.opts = append(l.opts, func(event *zerolog.Event) error {
		event.Str("pkg", pkg)
		return nil
	})
	return l
}

func (l *logger) WithFunc(funcName string) Logger {
	l.opts = append(l.opts, func(event *zerolog.Event) error {
		event.Str("func", funcName)
		return nil
	})
	return l
}

func (l *logger) Info(msg string) {
	l.Logger.Info().Msg(msg)
}

func (l *logger) Infof(msg string, args ...any) {
	l.Logger.Info().Msgf(msg, args...)
}

func (l *logger) Error(msg string) {
	l.Logger.Error().Stack().Msg(msg)
}

func (l *logger) Errorf(msg string, args ...any) {
	l.Logger.Error().Msgf(msg, args...)
}

func (l *logger) Debug(msg string) {
	l.Logger.Debug().Msg(msg)
}

func (l *logger) Debugf(msg string, args ...any) {
	l.Logger.Debug().Msgf(msg, args...)
}

func (l *logger) Warn(msg string) {
	l.Logger.Warn().Msg(msg)
}

func (l *logger) Warnf(msg string, args ...any) {
	l.Logger.Warn().Msgf(msg, args...)
}

func (l *logger) Fatal(msg string, err error) {
	l.Logger.Fatal().Stack().Err(err).Msg(msg)
}

func (l *logger) Fatalf(msg string, args ...any) {
	l.Logger.Fatal().Msgf(msg, args...)
}

func (l *logger) Panic(msg string) {
	l.Logger.Panic().Msg(msg)
}

func (l *logger) Panicf(msg string, args ...any) {
	l.Logger.Panic().Msgf(msg, args...)
}

func formatCaller(pc uintptr, file string, line int) string {
	return filepath.Base(file) + ":" + strconv.Itoa(line)
}
