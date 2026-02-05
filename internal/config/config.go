package config

import (
  "fmt"
  "os"
  "time"

  "github.com/BurntSushi/toml"
)

// default values for configuration
const (
  defaultPort              = 8080
  defaultShell             = "/bin/bash"
  defaultHookShell         = "/bin/sh"
  defaultCommandTimeout    = "5m"
  defaultCommandMaxTimeout = "30m"
  defaultIdleTimeout       = "30m"
  defaultShutdownTimeout   = "30s"
  defaultKillTimeout       = "5s"
)

// Config holds all configuration for shelld
type Config struct {
  Server  ServerConfig  `toml:"server"`
  Shell   ShellConfig   `toml:"shell"`
  Timeout TimeoutConfig `toml:"timeout"`
  Hooks   HooksConfig   `toml:"hooks"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
  Port        int   `toml:"port"`
  DieOnUnlock *bool `toml:"die_on_unlock"`
}

// ShellConfig holds shell execution configuration
type ShellConfig struct {
  Command          string `toml:"command"`
  WorkingDirectory string `toml:"working_directory"`
}

// TimeoutConfig holds all timeout configuration
type TimeoutConfig struct {
  Command        string `toml:"command"`
  CommandMaximum string `toml:"command_maximum"`
  Idle           string `toml:"idle"`
  Shutdown       string `toml:"shutdown"`
  Kill           string `toml:"kill"`

  // parsed durations
  CommandDuration        time.Duration `toml:"-"`
  CommandMaximumDuration time.Duration `toml:"-"`
  IdleDuration           time.Duration `toml:"-"`
  ShutdownDuration       time.Duration `toml:"-"`
  KillDuration           time.Duration `toml:"-"`
}

// HooksConfig holds lifecycle hook commands
type HooksConfig struct {
  Shell  string `toml:"shell"`
  Lock   string `toml:"lock"`
  Unlock string `toml:"unlock"`
}

// Load reads and parses a configuration file
func Load( path string ) ( *Config, error ) {
  data, err := os.ReadFile( path )
  if err != nil {
    return nil, fmt.Errorf( "The configuration file could not be read: %w", err )
  }

  cfg := &Config{}
  if err := toml.Unmarshal( data, cfg ); err != nil {
    return nil, fmt.Errorf( "The configuration file could not be parsed: %w", err )
  }

  applyDefaults( cfg )

  if err := parseDurations( cfg ); err != nil {
    return nil, err
  }

  if err := validate( cfg ); err != nil {
    return nil, err
  }

  return cfg, nil
}

// applyDefaults sets default values for unset configuration fields
func applyDefaults( cfg *Config ) {
  if cfg.Server.Port == 0 {
    cfg.Server.Port = defaultPort
  }
  if cfg.Shell.Command == "" {
    cfg.Shell.Command = defaultShell
  }
  if cfg.Timeout.Command == "" {
    cfg.Timeout.Command = defaultCommandTimeout
  }
  if cfg.Timeout.CommandMaximum == "" {
    cfg.Timeout.CommandMaximum = defaultCommandMaxTimeout
  }
  if cfg.Timeout.Idle == "" {
    cfg.Timeout.Idle = defaultIdleTimeout
  }
  if cfg.Timeout.Shutdown == "" {
    cfg.Timeout.Shutdown = defaultShutdownTimeout
  }
  if cfg.Timeout.Kill == "" {
    cfg.Timeout.Kill = defaultKillTimeout
  }
  if cfg.Hooks.Shell == "" {
    cfg.Hooks.Shell = defaultHookShell
  }
  if cfg.Server.DieOnUnlock == nil {
    defaultDieOnUnlock := true
    cfg.Server.DieOnUnlock = &defaultDieOnUnlock
  }
}

// parseDurations parses all duration string fields into time.Duration
func parseDurations( cfg *Config ) error {
  var err error

  cfg.Timeout.CommandDuration, err = time.ParseDuration( cfg.Timeout.Command )
  if err != nil {
    return fmt.Errorf( "The timeout.command value is invalid: %w", err )
  }

  cfg.Timeout.CommandMaximumDuration, err = time.ParseDuration( cfg.Timeout.CommandMaximum )
  if err != nil {
    return fmt.Errorf( "The timeout.command_maximum value is invalid: %w", err )
  }

  cfg.Timeout.IdleDuration, err = time.ParseDuration( cfg.Timeout.Idle )
  if err != nil {
    return fmt.Errorf( "The timeout.idle value is invalid: %w", err )
  }

  cfg.Timeout.ShutdownDuration, err = time.ParseDuration( cfg.Timeout.Shutdown )
  if err != nil {
    return fmt.Errorf( "The timeout.shutdown value is invalid: %w", err )
  }

  cfg.Timeout.KillDuration, err = time.ParseDuration( cfg.Timeout.Kill )
  if err != nil {
    return fmt.Errorf( "The timeout.kill value is invalid: %w", err )
  }

  return nil
}

// validate checks that required configuration values are set
func validate( cfg *Config ) error {
  if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
    return fmt.Errorf( "The server.port must be between 1 and 65535, but got %d.", cfg.Server.Port )
  }
  return nil
}
