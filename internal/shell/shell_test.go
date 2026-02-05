package shell

import (
  "log/slog"
  "os"
  "strings"
  "testing"
  "time"
)

func newTestShell( t *testing.T ) *Shell {
  t.Helper()
  logger := slog.New( slog.NewTextHandler( os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelError,
  } ) )
  return NewShell( "/bin/bash", "", 5*time.Second, logger )
}

func TestNewShell( t *testing.T ) {
  shell := newTestShell( t )

  if shell.State() != StateAvailable {
    t.Errorf( "The initial state should be Available, but got %s.", shell.State() )
  }
}

func TestShellStartAndState( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  if shell.State() != StateLocked {
    t.Errorf( "The state should be Ready after start, but got %s.", shell.State() )
  }
}

func TestShellDoubleStart( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  if err := shell.Start(); err == nil {
    t.Error( "The shell should return an error when started twice." )
  }
}

func TestShellRunCommand( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  output, err := shell.Execute( "echo hello", 30*time.Second )
  if err != nil {
    t.Fatalf( "The command failed to run: %v", err )
  }

  output = strings.TrimSpace( output )
  if output != "hello" {
    t.Errorf( "The output should be 'hello', but got '%s'.", output )
  }
}

func TestShellRunBeforeStart( t *testing.T ) {
  shell := newTestShell( t )

  _, err := shell.Execute( "echo hello", 30*time.Second )
  if err == nil {
    t.Error( "The shell should return an error when running a command before start." )
  }
}

func TestShellPersistence( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  _, err := shell.Execute( "export TEST_VAR=myvalue", 30*time.Second )
  if err != nil {
    t.Fatalf( "The export command failed to run: %v", err )
  }

  output, err := shell.Execute( "echo $TEST_VAR", 30*time.Second )
  if err != nil {
    t.Fatalf( "The echo command failed to run: %v", err )
  }

  output = strings.TrimSpace( output )
  if output != "myvalue" {
    t.Errorf( "The variable value should be 'myvalue', but got '%s'.", output )
  }
}

func TestShellKill( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  // start a long-running command that will timeout
  _, err := shell.Execute( "sleep 30", 100*time.Millisecond )
  if err != ErrTimeout {
    t.Fatalf( "The command should have timed out: %v", err )
  }

  // shell should be busy
  if shell.State() != StateExecuting {
    t.Errorf( "The state should be Busy after timeout, but got %s.", shell.State() )
  }

  // kill sends Ctrl+C to interrupt the command
  if err := shell.Kill(); err != nil {
    t.Fatalf( "The shell failed to kill: %v", err )
  }

  // wait for the interrupt to take effect
  time.Sleep( 500 * time.Millisecond )

  // shell should be ready again after interrupt
  if shell.State() != StateLocked {
    t.Errorf( "The state should be Ready after kill, but got %s.", shell.State() )
  }

  // verify shell still works
  output, err := shell.Execute( "echo still_alive", 30*time.Second )
  if err != nil {
    t.Fatalf( "The shell should still work after kill: %v", err )
  }
  if strings.TrimSpace( output ) != "still_alive" {
    t.Errorf( "The output should be 'still_alive', but got '%s'.", output )
  }
}

func TestShellKillWhenNotRunning( t *testing.T ) {
  shell := newTestShell( t )

  if err := shell.Kill(); err != nil {
    t.Errorf( "The shell should not return an error when killing a non-running shell: %v", err )
  }
}

func TestShellRecycle( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  _, err := shell.Execute( "export RECYCLE_TEST=before", 30*time.Second )
  if err != nil {
    t.Fatalf( "The command failed to run: %v", err )
  }

  // recycle: shutdown then start again ( this is what the server does )
  if err := shell.Unlock(); err != nil {
    t.Fatalf( "The shell failed to shutdown: %v", err )
  }

  if shell.State() != StateAvailable {
    t.Errorf( "The state should be Available after shutdown, but got %s.", shell.State() )
  }

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to restart: %v", err )
  }

  if shell.State() != StateLocked {
    t.Errorf( "The state should be Ready after restart, but got %s.", shell.State() )
  }

  output, err := shell.Execute( "echo ${RECYCLE_TEST:-unset}", 30*time.Second )
  if err != nil {
    t.Fatalf( "The command failed to run: %v", err )
  }

  output = strings.TrimSpace( output )
  if output != "unset" {
    t.Errorf( "The variable should be unset after recycle, but got '%s'.", output )
  }
}

func TestShellShutdown( t *testing.T ) {
  shell := newTestShell( t )

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  if err := shell.Unlock(); err != nil {
    t.Fatalf( "The shell failed to shutdown: %v", err )
  }

  if shell.State() != StateAvailable {
    t.Errorf( "The state should be Available after shutdown, but got %s.", shell.State() )
  }
}

func TestShellMultilineOutput( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  output, err := shell.Execute( "echo -e 'line1\\nline2\\nline3'", 30*time.Second )
  if err != nil {
    t.Fatalf( "The command failed to run: %v", err )
  }

  lines := strings.Split( strings.TrimSpace( output ), "\n" )
  if len( lines ) != 3 {
    t.Errorf( "The output should have 3 lines, but got %d: %v", len( lines ), lines )
  }
}

func TestShellOutputWithoutTrailingNewline( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  // printf without newline should not hang
  output, err := shell.Execute( "printf 'no_newline'", 5*time.Second )
  if err != nil {
    t.Fatalf( "The command failed to run: %v", err )
  }

  output = strings.TrimSpace( output )
  if output != "no_newline" {
    t.Errorf( "The output should be 'no_newline', but got '%s'.", output )
  }

  // head -c also outputs without trailing newline
  output, err = shell.Execute( "echo -n 'head_test'", 5*time.Second )
  if err != nil {
    t.Fatalf( "The echo -n command failed to run: %v", err )
  }

  output = strings.TrimSpace( output )
  if output != "head_test" {
    t.Errorf( "The output should be 'head_test', but got '%s'.", output )
  }
}

func TestShellTimeout( t *testing.T ) {
  shell := newTestShell( t )
  defer shell.Unlock()

  if err := shell.Start(); err != nil {
    t.Fatalf( "The shell failed to start: %v", err )
  }

  // run a command with a very short timeout
  _, err := shell.Execute( "sleep 5", 100*time.Millisecond )
  if err != ErrTimeout {
    t.Errorf( "The command should have timed out, but got: %v", err )
  }

  // shell should be busy
  if shell.State() != StateExecuting {
    t.Errorf( "The state should be Busy after timeout, but got %s.", shell.State() )
  }

  // wait for command to complete in background
  time.Sleep( 6 * time.Second )

  // shell should be ready again
  if shell.State() != StateLocked {
    t.Errorf( "The state should be Ready after background completion, but got %s.", shell.State() )
  }
}
