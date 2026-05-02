package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

func Run(ctx context.Context, port string, handler http.Handler) error {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()

		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(sctx)
		if err != nil {
			log.Println("Shutdown error:", err)
		}
	}()

	log.Println("Listening on :" + port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
