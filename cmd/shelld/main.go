package main

import (
  "context"
  "flag"
  "fmt"
  "io"
  "log/slog"
  "net"
  "net/http"
  "os"
  "os/signal"
  "sync"
  "syscall"
  "time"

  "github.com/endless/shelld/internal/config"
  "github.com/endless/shelld/internal/lifecycle"
  "github.com/endless/shelld/internal/shell"
)

type serverInstance struct {
  cfg           *config.Config
  shell         *shell.Shell
  hooks         *lifecycle.Hooks
  logger        *slog.Logger
  lastActivity  time.Time
  activityMutex sync.Mutex
  key     string
  keyMutex      sync.RWMutex
}

func main() {
  configPath := flag.String( "config", "", "path to configuration file" )
  flag.Parse()

  // env var can override flag
  if *configPath == "" {
    *configPath = os.Getenv( "SHELLD_CONFIG" )
  }
  if *configPath == "" {
    fmt.Fprintln( os.Stderr,
                  "The configuration file path is required ( use --config or SHELLD_CONFIG )." )
    os.Exit( 1 )
  }

  logger := slog.New( slog.NewJSONHandler( os.Stdout,
                                           &slog.HandlerOptions{ Level: slog.LevelDebug } ) )

  cfg, err := config.Load( *configPath )
  if err != nil {
    logger.Error( "Server | Main | The configuration could not be loaded.", "error", err )
    os.Exit( 1 )
  }

  server := &serverInstance{
    cfg: cfg,
    shell: shell.NewShell(
      cfg.Shell.Command,
      cfg.Shell.WorkingDirectory,
      cfg.Timeout.KillDuration,
      logger,
    ),
    hooks: lifecycle.NewHooks(
      cfg.Hooks.Shell,
      cfg.Hooks.Lock,
      cfg.Hooks.Unlock,
      logger,
    ),
    logger:       logger,
    lastActivity: time.Now(),
  }

  multiplexer := http.NewServeMux()
  multiplexer.HandleFunc( "POST /lock", server.setKeyMiddleware( server.handleLock ) )
  multiplexer.HandleFunc( "POST /execute", server.verifyKeyMiddleware( server.handleExecute ) )
  multiplexer.HandleFunc( "POST /kill", server.verifyKeyMiddleware( server.handleKill ) )
  multiplexer.HandleFunc( "POST /unlock", server.verifyKeyMiddleware( server.handleUnlock ) )
  multiplexer.HandleFunc( "GET /output", server.verifyKeyMiddleware( server.handleOutput ) )
  multiplexer.HandleFunc( "GET /state", server.verifyKeyMiddleware( server.handleState ) )
  multiplexer.HandleFunc( "GET /health", server.handleHealth )

  httpServer := &http.Server{
    Addr:        fmt.Sprintf( ":%d", cfg.Server.Port ),
    Handler:     multiplexer,
    ReadTimeout: 30 * time.Second,
  }

  ctx, cancel := context.WithCancel( context.Background() )
  defer cancel()

  go server.monitorIdleTimeout( ctx, httpServer )

  shutdownChannel := make( chan os.Signal, 1 )
  signal.Notify( shutdownChannel, syscall.SIGINT, syscall.SIGTERM )

  go func() {
    <-shutdownChannel
    logger.Info( "Server | Main | The server received a shutdown signal." )
    cancel()

    shutdownCtx, shutdownCancel := context.WithTimeout( context.Background(),
                                                        cfg.Timeout.ShutdownDuration )
    defer shutdownCancel()

    server.hooks.RunUnlock( shutdownCtx, server.key )
    server.shell.Unlock()
    httpServer.Shutdown( shutdownCtx )
  }()

  listener, err := net.Listen( "tcp", fmt.Sprintf( ":%d", cfg.Server.Port ) )
  if err != nil {
    logger.Error( "Server | Main | The server could not bind to port.", "port", cfg.Server.Port, "error", err )
    os.Exit( 1 )
  }

  logger.Info( "Server | Main | The server is ready.", "port", cfg.Server.Port )

  if err := httpServer.Serve( listener ); err != nil && err != http.ErrServerClosed {
    logger.Error( "Server | Main | The server encountered an error.", "error", err )
    os.Exit( 1 )
  }

  logger.Info( "Server | Main | The server has stopped." )
}

func ( server *serverInstance ) setKeyMiddleware( next http.HandlerFunc ) http.HandlerFunc {
  return func( writer http.ResponseWriter, request *http.Request ) {
    providedKey := request.Header.Get( "X-Shell-Key" )
    if providedKey == "" {
      http.Error( writer, "The X-Shell-Key header is required.", http.StatusUnauthorized )
      return
    }

    server.keyMutex.Lock()
    if server.key == "" {
      // first startup locks the shell to this key
      server.key = providedKey
      server.logger.Info( "Server | Auth | The shell has been locked to a key." )
    }
    key := server.key
    server.keyMutex.Unlock()

    if providedKey != key {
      http.Error( writer, "The provided key does not match the locked key.", http.StatusUnauthorized )
      return
    }

    server.updateActivity()
    next( writer, request )
  }
}

func ( server *serverInstance ) verifyKeyMiddleware( next http.HandlerFunc ) http.HandlerFunc {
  return func( writer http.ResponseWriter, request *http.Request ) {
    providedKey := request.Header.Get( "X-Shell-Key" )
    if providedKey == "" {
      http.Error( writer, "The X-Shell-Key header is required.", http.StatusUnauthorized )
      return
    }

    server.keyMutex.RLock()
    key := server.key
    server.keyMutex.RUnlock()

    if key == "" {
      http.Error( writer, "The shell has not been locked.", http.StatusConflict )
      return
    }

    if providedKey != key {
      http.Error( writer, "The provided key does not match the locked key.", http.StatusUnauthorized )
      return
    }

    server.updateActivity()
    next( writer, request )
  }
}

func ( server *serverInstance ) handleLock( writer http.ResponseWriter,
                                            request *http.Request ) {
  server.hooks.RunLock( request.Context(), server.key )

  if err := server.shell.Start(); err != nil {
    state := server.shell.State()
    if state == shell.StateLocked || state == shell.StateExecuting {
      http.Error( writer, "The shell is already locked.", http.StatusConflict )
    } else if state == shell.StateUnrecoverable {
      http.Error( writer, "The shell is in an unrecoverable state.", http.StatusConflict )
    } else {
      http.Error( writer, "The shell could not be started.", http.StatusInternalServerError )
    }
    return
  }

  writer.WriteHeader( http.StatusOK )
}

func ( server *serverInstance ) handleExecute( writer http.ResponseWriter,
                                               request *http.Request ) {
  body, err := io.ReadAll( request.Body )
  if err != nil {
    http.Error( writer, "The request body could not be read.", http.StatusBadRequest )
    return
  }
  defer request.Body.Close()

  command := string( body )
  if command == "" {
    http.Error( writer, "The command cannot be empty.", http.StatusBadRequest )
    return
  }

  timeout := server.cfg.Timeout.CommandDuration
  if timeoutHeader := request.Header.Get( "X-Command-Timeout" ); timeoutHeader != "" {
    parsedTimeout, err := time.ParseDuration( timeoutHeader )
    if err != nil {
      http.Error( writer, "The X-Command-Timeout header is invalid.", http.StatusBadRequest )
      return
    }
    if parsedTimeout > server.cfg.Timeout.CommandMaximumDuration {
      parsedTimeout = server.cfg.Timeout.CommandMaximumDuration
    }
    if parsedTimeout > 0 {
      timeout = parsedTimeout
    }
  }

  output, err := server.shell.Execute( command, timeout )
  if err != nil {
    if err == shell.ErrTimeout {
      http.Error( writer, "The command timed out. The shell is busy and the command is still running.",
                  http.StatusAccepted )
      return
    }
    state := server.shell.State()
    if state == shell.StateAvailable {
      http.Error( writer, "The shell has not been locked.", http.StatusConflict )
    } else if state == shell.StateExecuting {
      http.Error( writer, "The shell is busy executing another command.", http.StatusConflict )
    } else if state == shell.StateUnrecoverable {
      http.Error( writer, "The shell is in an unrecoverable state.", http.StatusConflict )
    } else {
      http.Error( writer, "The command could not be executed.", http.StatusInternalServerError )
    }
    return
  }

  writer.WriteHeader( http.StatusOK )
  writer.Write( []byte( output ) )
}

func ( server *serverInstance ) handleKill( writer http.ResponseWriter,
                                            request *http.Request ) {
  if err := server.shell.Kill(); err != nil {
    http.Error( writer, "The shell could not be killed.", http.StatusInternalServerError )
    return
  }

  writer.WriteHeader( http.StatusOK )
}

func ( server *serverInstance ) handleUnlock( writer http.ResponseWriter,
                                              request *http.Request ) {
  if *server.cfg.Server.DieOnUnlock {
    // shutdown mode: terminate the server ( signal handler will run unlock hook )
    writer.WriteHeader( http.StatusOK )

    // allow response to be sent before triggering shutdown
    go func() {
      time.Sleep( 100 * time.Millisecond )
      syscall.Kill( syscall.Getpid(), syscall.SIGTERM )
    }()
  } else {
    // recycle mode: terminate shell, clear key, stay running for next client
    server.hooks.RunUnlock( request.Context(), server.key )
    server.shell.Unlock()

    server.keyMutex.Lock()
    server.key = ""
    server.keyMutex.Unlock()

    server.logger.Info( "Server | Unlock | The shell has been recycled and is available for a new client." )
    writer.WriteHeader( http.StatusOK )
  }
}

func ( server *serverInstance ) handleOutput( writer http.ResponseWriter,
                                              request *http.Request ) {
  writer.WriteHeader( http.StatusOK )
  writer.Write( []byte( server.shell.Output() ) )
}

func ( server *serverInstance ) handleState( writer http.ResponseWriter,
                                             request *http.Request ) {
  writer.WriteHeader( http.StatusOK )
  writer.Write( []byte( server.shell.State() ) )
}

func ( server *serverInstance ) handleHealth( writer http.ResponseWriter,
                                              request *http.Request ) {
  writer.WriteHeader( http.StatusOK )
}

func ( server *serverInstance ) updateActivity() {
  server.activityMutex.Lock()
  defer server.activityMutex.Unlock()
  server.lastActivity = time.Now()
}

func ( server *serverInstance ) monitorIdleTimeout( ctx context.Context,
                                                    httpServer *http.Server ) {
  ticker := time.NewTicker( 30 * time.Second )
  defer ticker.Stop()

  for {
    select {
    case <-ctx.Done():
      return
    case <-ticker.C:
      server.activityMutex.Lock()
      idleDuration := time.Since( server.lastActivity )
      server.activityMutex.Unlock()

      if idleDuration > server.cfg.Timeout.IdleDuration {
        server.logger.Info( "Server | MonitorIdleTimeout | The server is shutting down due to idle timeout.",
                            "idle_duration", idleDuration )
        syscall.Kill( syscall.Getpid(), syscall.SIGTERM )
        return
      }
    }
  }
}
