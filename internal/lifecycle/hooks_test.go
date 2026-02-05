package lifecycle

import (
  "context"
  "log/slog"
  "os"
  "testing"
  "time"
)

func newTestHooks( lock, unlock string ) *Hooks {
  logger := slog.New( slog.NewTextHandler( os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelError,
  } ) )
  return NewHooks( "/bin/sh", lock, unlock, logger )
}

func TestNewHooks( t *testing.T ) {
  hooks := newTestHooks( "echo lock", "echo unlock" )

  if hooks.lock != "echo lock" {
    t.Errorf( "The lock hook should be 'echo lock', but got '%s'.", hooks.lock )
  }
  if hooks.unlock != "echo unlock" {
    t.Errorf( "The unlock hook should be 'echo unlock', but got '%s'.", hooks.unlock )
  }
}

func TestRunWithEmptyCommand( t *testing.T ) {
  hooks := newTestHooks( "", "" )
  ctx := context.Background()

  hooks.RunLock( ctx, "test-key" )
  hooks.RunUnlock( ctx, "test-key" )
}

func TestRunLockExecutesCommand( t *testing.T ) {
  tmpFile := t.TempDir() + "/lock_marker"
  hooks := newTestHooks( "touch "+tmpFile, "" )

  ctx := context.Background()
  hooks.RunLock( ctx, "test-key" )

  time.Sleep( 100 * time.Millisecond )

  if _, err := os.Stat( tmpFile ); os.IsNotExist( err ) {
    t.Error( "The lock hook should have created the marker file." )
  }
}

func TestRunUnlockExecutesCommand( t *testing.T ) {
  tmpFile := t.TempDir() + "/unlock_marker"
  hooks := newTestHooks( "", "touch "+tmpFile )

  ctx := context.Background()
  hooks.RunUnlock( ctx, "test-key" )

  time.Sleep( 100 * time.Millisecond )

  if _, err := os.Stat( tmpFile ); os.IsNotExist( err ) {
    t.Error( "The unlock hook should have created the marker file." )
  }
}

func TestHookContextCancellation( t *testing.T ) {
  hooks := newTestHooks( "sleep 10", "" )

  ctx, cancel := context.WithTimeout( context.Background(), 100*time.Millisecond )
  defer cancel()

  start := time.Now()
  hooks.RunLock( ctx, "test-key" )
  elapsed := time.Since( start )

  if elapsed > 500*time.Millisecond {
    t.Errorf( "The hook should have been cancelled quickly, but took %v.", elapsed )
  }
}

func TestHookEnvironmentVariable( t *testing.T ) {
  tmpFile := t.TempDir() + "/env_test"
  hooks := newTestHooks( "echo $SHELLD_KEY > "+tmpFile, "" )

  ctx := context.Background()
  hooks.RunLock( ctx, "my-secret-key" )

  time.Sleep( 100 * time.Millisecond )

  content, err := os.ReadFile( tmpFile )
  if err != nil {
    t.Fatalf( "The environment test file could not be read: %v", err )
  }

  if string( content ) != "my-secret-key\n" {
    t.Errorf( "The SHELLD_KEY variable should be 'my-secret-key', but got '%s'.", string( content ) )
  }
}

func TestHookWithCustomShell( t *testing.T ) {
  logger := slog.New( slog.NewTextHandler( os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelError,
  } ) )

  tmpFile := t.TempDir() + "/bash_test"
  hooks := NewHooks( "/bin/bash", "touch "+tmpFile, "", logger )

  ctx := context.Background()
  hooks.RunLock( ctx, "test-key" )

  time.Sleep( 100 * time.Millisecond )

  if _, err := os.Stat( tmpFile ); os.IsNotExist( err ) {
    t.Error( "The hook with custom shell should have executed." )
  }
}
