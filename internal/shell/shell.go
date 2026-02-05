package shell

import (
  "bytes"
  "encoding/base64"
  "fmt"
  "io"
  "log/slog"
  "os"
  "os/exec"
  "strings"
  "sync"
  "syscall"
  "time"

  "github.com/creack/pty"
)

// State represents the current state of the shell
type State string

const (
  StateAvailable     State = "available"     // initial state, no shell running
  StateLocked        State = "locked"        // shell running, waiting for commands
  StateExecuting     State = "executing"    // shell executing a command
  StateUnrecoverable State = "unrecoverable" // shell in error state, needs recycle
)

// ErrTimeout is returned when a command times out waiting for completion
var ErrTimeout = fmt.Errorf( "The command timed out waiting for completion." )

// Shell manages a persistent shell session with PTY
type Shell struct {
  mu                sync.Mutex
  state             State
  cmd               *exec.Cmd
  ptyFile           *os.File
  outputBuffer      *bytes.Buffer
  killGracePeriod   time.Duration
  shellCommand      string
  workingDirectory  string
  logger            *slog.Logger
  lastOutput        string
  commandDone       chan error
  currentCommand    string
  startMarker       string
  endMarker         string
}

// NewShell creates a new shell manager
func NewShell( shellCommand string,
               workingDirectory string,
               killGracePeriod time.Duration,
               logger *slog.Logger ) *Shell {
  return &Shell{
    state:            StateAvailable,
    killGracePeriod:  killGracePeriod,
    shellCommand:     shellCommand,
    workingDirectory: workingDirectory,
    logger:           logger,
    outputBuffer:     &bytes.Buffer{},
  }
}

// State returns the current shell state
func ( shell *Shell ) State() State {
  shell.mu.Lock()
  defer shell.mu.Unlock()
  return shell.state
}

// Start initializes the shell with a PTY
func ( shell *Shell ) Start() error {
  shell.mu.Lock()
  defer shell.mu.Unlock()

  if shell.state != StateAvailable {
    return fmt.Errorf( "The shell cannot be started from state %s.", shell.state )
  }

  shell.logger.Info( "Shell | Start | The shell is starting.", "command", shell.shellCommand )

  cmd := exec.Command( shell.shellCommand )
  cmd.Env = append( os.Environ(), "TERM=xterm-256color", )

  if shell.workingDirectory != "" {
    cmd.Dir = shell.workingDirectory
  }

  ptyFile, err := pty.Start( cmd )
  if err != nil {
    shell.state = StateUnrecoverable
    return fmt.Errorf( "The PTY could not be allocated: %w", err )
  }

  shell.cmd = cmd
  shell.ptyFile = ptyFile
  shell.outputBuffer.Reset()

  // verify shell is ready using a marker echo
  readyMarker := fmt.Sprintf( "<<<SHELLD_READY_%d>>>", time.Now().UnixNano() )
  shell.ptyFile.Write( []byte( fmt.Sprintf( "echo '%s'\n", readyMarker ) ) )

  if err := shell.waitForOutput( readyMarker, 30*time.Second ); err != nil {
    shell.cleanup()
    shell.state = StateUnrecoverable
    return fmt.Errorf( "The shell failed to initialize: %w", err )
  }

  shell.outputBuffer.Reset()
  shell.state = StateLocked
  shell.logger.Info( "Shell | Start | The shell is ready." )
  return nil
}

// Execute runs a command in the shell and returns its output
func ( shell *Shell ) Execute( command string, timeout time.Duration ) ( string, error ) {
  shell.mu.Lock()

  if shell.state != StateLocked {
    shell.mu.Unlock()
    return "", fmt.Errorf( "The shell is not ready ( state: %s ).", shell.state )
  }

  shell.state = StateExecuting
  shell.outputBuffer.Reset()
  shell.lastOutput = ""
  shell.currentCommand = command
  shell.commandDone = make( chan error, 1 )

  // generate unique start and end markers for this command
  // this eliminates reliance on prompt detection which has timing issues
  markerID := time.Now().UnixNano()
  shell.startMarker = fmt.Sprintf( "<<<SHELLD_START_%d>>>", markerID )
  shell.endMarker = fmt.Sprintf( "<<<SHELLD_END_%d>>>", markerID )

  shell.logger.Debug( "Shell | Run | Executing command.", "command", command, "timeout", timeout )

  // the command is wrapped with start and end markers; the output between these markers is the actual
  // command output that is returned to the caller

  // the command is base64 encoded to handle heredocs and other multiline constructs that require
  // newlines ( can't just replace with semicolons )

  // the extra echo before end marker ensures there's always a newline even if the command output
  // doesn't have a trailing newline ( e.g. printf 'foo' )
  encodedCmd := base64.StdEncoding.EncodeToString( []byte( command ) )
  wrappedCmd := fmt.Sprintf( "echo '%s';eval \"$(echo '%s'|base64 -d)\";echo;echo '%s'\n",
                             shell.startMarker, encodedCmd, shell.endMarker )
  _, err := shell.ptyFile.Write( []byte( wrappedCmd ) )
  if err != nil {
    shell.state = StateUnrecoverable
    shell.mu.Unlock()
    return "", fmt.Errorf( "The command could not be written to the shell: %w", err )
  }

  // start background reader
  go shell.readUntilMarker()

  shell.mu.Unlock()

  // wait for completion or timeout
  select {
  case err := <-shell.commandDone:
    shell.mu.Lock()
    defer shell.mu.Unlock()

    if err != nil {
      shell.state = StateUnrecoverable
      return "", err
    }

    output := shell.lastOutput
    shell.state = StateLocked
    return output, nil

  case <-time.After( timeout ):
    // timeout - shell stays busy, reader continues in background
    return "", ErrTimeout
  }
}

// Output returns the output from the last completed command
func ( shell *Shell ) Output() string {
  shell.mu.Lock()
  defer shell.mu.Unlock()
  return shell.lastOutput
}

// Kill interrupts the current command by sending Ctrl+C to the PTY
// the shell remains running and ready for new commands
func ( shell *Shell ) Kill() error {
  shell.mu.Lock()
  defer shell.mu.Unlock()

  if shell.ptyFile == nil {
    return nil
  }

  shell.logger.Info( "Shell | Kill | Sending interrupt to current command." )

  // send Ctrl+C (ETX, 0x03) to interrupt the current command
  _, err := shell.ptyFile.Write( []byte{ 0x03 } )
  if err != nil {
    return fmt.Errorf( "The interrupt could not be sent: %w", err )
  }

  // if shell was busy, it will return to ready when the command is interrupted
  // the background reader will handle the state transition
  if shell.state == StateExecuting {
    shell.logger.Debug( "Shell | Kill | Waiting for command to be interrupted." )
  }

  return nil
}

// Unlock gracefully terminates the shell
func ( shell *Shell ) Unlock() error {
  shell.mu.Lock()
  defer shell.mu.Unlock()

  if shell.cmd == nil || shell.cmd.Process == nil {
    shell.state = StateAvailable
    return nil
  }

  shell.logger.Info( "Shell | Unlock | The shell is shutting down." )

  process := shell.cmd.Process

  if shell.ptyFile != nil {
    shell.ptyFile.Write( []byte( "exit\n" ) )
    shell.ptyFile.Close()
    shell.ptyFile = nil
  }

  done := make( chan struct{} )
  go func() {
    shell.cmd.Wait()
    close( done )
  }()

  select {
  case <-done:
    shell.logger.Debug( "Shell | Unlock | The shell exited cleanly." )
  case <-time.After( shell.killGracePeriod ):
    shell.logger.Debug( "Shell | Unlock | Clean exit timeout, forcing termination." )
    process.Signal( syscall.SIGKILL )
    <-done
  }

  shell.cmd = nil
  shell.outputBuffer.Reset()
  shell.state = StateAvailable
  return nil
}

// readUntilMarker reads from PTY until the end marker output is found
func ( shell *Shell ) readUntilMarker() {
  buf := make( []byte, 4096 )
  // look for end marker as OUTPUT (at start of line, followed by newline)
  // this distinguishes from the end marker appearing in the command echo
  endMarkerOutput := []byte( "\n" + shell.endMarker + "\r\n" )

  for {
    shell.mu.Lock()
    if shell.ptyFile == nil {
      shell.mu.Unlock()
      shell.commandDone <- fmt.Errorf( "The shell was closed." )
      return
    }
    ptyFile := shell.ptyFile
    shell.mu.Unlock()

    bytesRead, err := ptyFile.Read( buf )
    if err != nil {
      if err == io.EOF {
        shell.commandDone <- fmt.Errorf( "The shell process terminated unexpectedly." )
      } else {
        shell.commandDone <- fmt.Errorf( "The shell read failed: %w", err )
      }
      return
    }

    shell.mu.Lock()
    shell.outputBuffer.Write( buf[:bytesRead] )
    bufferBytes := shell.outputBuffer.Bytes()

    if bytes.Contains( bufferBytes, endMarkerOutput ) {
      shell.lastOutput = shell.extractOutput( shell.currentCommand )
      shell.logger.Debug( "Shell | ReadUntilMarker | The command completed.",
                          "output_length", len( shell.lastOutput ) )
      // update state to ready here in case Run() has already timed out
      shell.state = StateLocked
      shell.mu.Unlock()
      shell.commandDone <- nil
      return
    }
    shell.mu.Unlock()
  }
}

// waitForOutput waits for a specific string to appear in the output
func ( shell *Shell ) waitForOutput( marker string, timeout time.Duration ) error {
  done := make( chan error, 1 )
  markerBytes := []byte( marker )

  go func() {
    buf := make( []byte, 4096 )

    for {
      bytesRead, err := shell.ptyFile.Read( buf )
      if err != nil {
        if err == io.EOF {
          done <- fmt.Errorf( "The shell process terminated unexpectedly." )
        } else {
          done <- fmt.Errorf( "The shell read failed: %w", err )
        }
        return
      }

      shell.outputBuffer.Write( buf[:bytesRead] )

      if bytes.Contains( shell.outputBuffer.Bytes(), markerBytes ) {
        done <- nil
        return
      }
    }
  }()

  select {
  case err := <-done:
    return err
  case <-time.After( timeout ):
    shell.logger.Error( "Shell | WaitForOutput | The marker was not found.",
                        "timeout", timeout,
                        "marker", marker )
    return ErrTimeout
  }
}

// extractOutput extracts the command output from between the start and end markers
func ( shell *Shell ) extractOutput( command string ) string {
  output := shell.outputBuffer.String()

  shell.logger.Debug( "Shell | ExtractOutput | Raw buffer.",
                      "raw", output,
                      "startMarker", shell.startMarker,
                      "endMarker", shell.endMarker )

  // find the start marker output (not the echo, but the actual output)
  startMarkerOutput := shell.startMarker + "\r\n"
  startIdx := strings.Index( output, startMarkerOutput )
  if startIdx == -1 {
    shell.logger.Debug( "Shell | ExtractOutput | Start marker not found." )
    return ""
  }

  // take everything after the start marker
  output = output[startIdx+len( startMarkerOutput ):]

  // find the end marker output
  endIdx := strings.Index( output, shell.endMarker )
  if endIdx == -1 {
    shell.logger.Debug( "Shell | ExtractOutput | End marker not found." )
    return ""
  }

  // take everything before the end marker
  output = output[:endIdx]

  // clean up lines
  lines := strings.Split( output, "\n" )
  var cleanLines []string
  for _, line := range lines {
    line = strings.TrimSuffix( line, "\r" )
    if line != "" {
      cleanLines = append( cleanLines, line )
    }
  }

  result := strings.Join( cleanLines, "\n" )
  shell.logger.Debug( "Shell | ExtractOutput | Final result.",
                      "result", result,
                      "cleanLines", cleanLines )
  return result
}

// cleanup releases PTY and process resources
func ( shell *Shell ) cleanup() {
  if shell.ptyFile != nil {
    shell.ptyFile.Close()
    shell.ptyFile = nil
  }
  shell.cmd = nil
  shell.outputBuffer.Reset()
}
