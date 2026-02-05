package lifecycle

import (
  "context"
  "log/slog"
  "os"
  "os/exec"
)

// Hooks manages lifecycle hook execution
type Hooks struct {
  shellCommand string
  lock         string
  unlock       string
  logger       *slog.Logger
}

// NewHooks creates a new Hooks manager
func NewHooks( shellCommand string,
               lock string,
               unlock string,
               logger *slog.Logger ) *Hooks {
  return &Hooks{
    shellCommand: shellCommand,
    lock:         lock,
    unlock:       unlock,
    logger:       logger,
  }
}

// RunLock executes the lock hook if configured
func ( hooks *Hooks ) RunLock( ctx context.Context, key string ) {
  hooks.run( ctx, "lock", hooks.lock, key )
}

// RunUnlock executes the unlock hook if configured
func ( hooks *Hooks ) RunUnlock( ctx context.Context, key string ) {
  hooks.run( ctx, "unlock", hooks.unlock, key )
}

// run executes a hook command in a separate process
func ( hooks *Hooks ) run( ctx context.Context, hookName string, command string, key string ) {
  if command == "" {
    return
  }

  hooks.logger.Info( "Lifecycle | Run | Executing hook.",
                     "hook", hookName,
                     "command", command )

  cmd := exec.CommandContext( ctx, hooks.shellCommand, "-c", command )
  cmd.Env = append( os.Environ(), "SHELLD_KEY="+key )
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  if err := cmd.Run(); err != nil {
    // hooks are non-blocking - log errors but don't fail
    hooks.logger.Error( "Lifecycle | Run | The hook failed.",
                        "hook", hookName,
                        "command", command,
                        "error", err )
  } else {
    hooks.logger.Info( "Lifecycle | Run | The hook completed.", "hook", hookName )
  }
}
