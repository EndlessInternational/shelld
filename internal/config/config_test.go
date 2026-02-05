package config

import (
  "os"
  "path/filepath"
  "testing"
)

func TestLoadWithDefaults( t *testing.T ) {
  content := `
# minimal config - just need valid TOML
`
  path := writeTempConfig( t, content )

  cfg, err := Load( path )
  if err != nil {
    t.Fatalf( "The configuration could not be loaded: %v", err )
  }

  if cfg.Server.Port != defaultPort {
    t.Errorf( "The default port should be %d, but got %d.", defaultPort, cfg.Server.Port )
  }
  if cfg.Shell.Command != defaultShell {
    t.Errorf( "The default shell should be %s, but got %s.", defaultShell, cfg.Shell.Command )
  }
  if cfg.Timeout.Command != defaultCommandTimeout {
    t.Errorf( "The default command timeout should be %s, but got %s.", defaultCommandTimeout, cfg.Timeout.Command )
  }
  if cfg.Timeout.CommandMaximum != defaultCommandMaxTimeout {
    t.Errorf( "The default command maximum timeout should be %s, but got %s.", defaultCommandMaxTimeout, cfg.Timeout.CommandMaximum )
  }
  if cfg.Timeout.Idle != defaultIdleTimeout {
    t.Errorf( "The default idle timeout should be %s, but got %s.", defaultIdleTimeout, cfg.Timeout.Idle )
  }
  if cfg.Timeout.Shutdown != defaultShutdownTimeout {
    t.Errorf( "The default shutdown timeout should be %s, but got %s.", defaultShutdownTimeout, cfg.Timeout.Shutdown )
  }
  if cfg.Timeout.Kill != defaultKillTimeout {
    t.Errorf( "The default kill timeout should be %s, but got %s.", defaultKillTimeout, cfg.Timeout.Kill )
  }
  if cfg.Hooks.Shell != defaultHookShell {
    t.Errorf( "The default hook shell should be %s, but got %s.", defaultHookShell, cfg.Hooks.Shell )
  }
}

func TestLoadWithCustomValues( t *testing.T ) {
  content := `
[server]
port = 9000

[shell]
command = "/bin/zsh"

[timeout]
command = "10m"
command_maximum = "1h"
idle = "1h"
shutdown = "1m"
kill = "10s"

[hooks]
shell = "/bin/bash"
lock = "echo locking"
unlock = "echo unlocking"
`
  path := writeTempConfig( t, content )

  cfg, err := Load( path )
  if err != nil {
    t.Fatalf( "The configuration could not be loaded: %v", err )
  }

  if cfg.Server.Port != 9000 {
    t.Errorf( "The port should be 9000, but got %d.", cfg.Server.Port )
  }
  if cfg.Shell.Command != "/bin/zsh" {
    t.Errorf( "The shell should be /bin/zsh, but got %s.", cfg.Shell.Command )
  }
  if cfg.Timeout.Command != "10m" {
    t.Errorf( "The command timeout should be 10m, but got %s.", cfg.Timeout.Command )
  }
  if cfg.Timeout.CommandMaximum != "1h" {
    t.Errorf( "The command maximum timeout should be 1h, but got %s.", cfg.Timeout.CommandMaximum )
  }
  if cfg.Timeout.Kill != "10s" {
    t.Errorf( "The kill timeout should be 10s, but got %s.", cfg.Timeout.Kill )
  }
  if cfg.Hooks.Lock != "echo locking" {
    t.Errorf( "The lock hook should be 'echo locking', but got %s.", cfg.Hooks.Lock )
  }
}

func TestLoadInvalidPort( t *testing.T ) {
  content := `
[server]
port = 99999
`
  path := writeTempConfig( t, content )

  _, err := Load( path )
  if err == nil {
    t.Fatal( "The configuration should fail to load when the port is invalid." )
  }
}

func TestLoadInvalidDuration( t *testing.T ) {
  content := `
[timeout]
command = "invalid"
`
  path := writeTempConfig( t, content )

  _, err := Load( path )
  if err == nil {
    t.Fatal( "The configuration should fail to load when a duration is invalid." )
  }
}

func TestLoadNonexistentFile( t *testing.T ) {
  _, err := Load( "/nonexistent/path/config.toml" )
  if err == nil {
    t.Fatal( "The configuration should fail to load when the file does not exist." )
  }
}

func TestDurationParsing( t *testing.T ) {
  content := `
[timeout]
command = "5m"
command_maximum = "30m"
idle = "30m"
shutdown = "30s"
kill = "10s"
`
  path := writeTempConfig( t, content )

  cfg, err := Load( path )
  if err != nil {
    t.Fatalf( "The configuration could not be loaded: %v", err )
  }

  if cfg.Timeout.CommandDuration.Minutes() != 5 {
    t.Errorf( "The command timeout should be 5m, but got %v.", cfg.Timeout.CommandDuration )
  }
  if cfg.Timeout.CommandMaximumDuration.Minutes() != 30 {
    t.Errorf( "The command maximum timeout should be 30m, but got %v.", cfg.Timeout.CommandMaximumDuration )
  }
  if cfg.Timeout.IdleDuration.Minutes() != 30 {
    t.Errorf( "The idle timeout should be 30m, but got %v.", cfg.Timeout.IdleDuration )
  }
  if cfg.Timeout.ShutdownDuration.Seconds() != 30 {
    t.Errorf( "The shutdown timeout should be 30s, but got %v.", cfg.Timeout.ShutdownDuration )
  }
  if cfg.Timeout.KillDuration.Seconds() != 10 {
    t.Errorf( "The kill timeout should be 10s, but got %v.", cfg.Timeout.KillDuration )
  }
}

func writeTempConfig( t *testing.T, content string ) string {
  t.Helper()
  dir := t.TempDir()
  path := filepath.Join( dir, "config.toml" )
  if err := os.WriteFile( path, []byte( content ), 0644 ); err != nil {
    t.Fatalf( "The temporary config file could not be written: %v", err )
  }
  return path
}
