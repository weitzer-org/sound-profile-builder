package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// shutdownTimeout bounds how long serve waits for in-flight requests after a
// stop signal. Fly's kill_timeout grace period applies on top of this.
const shutdownTimeout = 8 * time.Second

// serve runs srv until ctx is done or the listener fails. On ctx done it first
// marks every in-memory running preset run abandoned (so a stopped server
// reports a clear verdict instead of a run that looks alive), then shuts the
// HTTP server down gracefully.
func serve(ctx context.Context, srv *http.Server, svc *presetsvc.Service) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	n := svc.AbandonActive("server stopped mid-run (Fly auto-stop or deploy); start it again")
	log.Printf("Shutdown: abandoned %d active run(s)", n)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
