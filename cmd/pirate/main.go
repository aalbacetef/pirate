package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/aalbacetef/pirate"
)

const RequestTimeout = 5 * time.Minute

func main() {
	cfgPath := ""

	flag.StringVar(&cfgPath, "config", cfgPath, "Specify a configuration file")

	flag.Parse()

	if err := run(cfgPath); err != nil {
		fmt.Println("error: ", err)
	}
}

func run(cfgPath string) error {
	cfg, src, err := pirate.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("could not load config (source='%s'): %w", src, err)
	}

	srv, err := pirate.NewServer(cfg)
	if err != nil {
		if srv != nil {
			srv.Close()
		}

		return fmt.Errorf("pirate.NewServer: %w", err)
	}
	defer srv.Close()

	router := chi.NewRouter()
	router.Post("/*", srv.HandleRequest)

	addr := fmt.Sprintf("localhost:%d", cfg.Server.Port)
	fmt.Println("listening on: ", addr)

	httpSrv := &http.Server{
		Addr:        addr,
		ReadTimeout: RequestTimeout,
		Handler:     router,
	}

	if listenErr := httpSrv.ListenAndServe(); listenErr != nil {
		if !errors.Is(listenErr, http.ErrServerClosed) {
			return fmt.Errorf("ListenAndServe: %w", listenErr)
		}
	}

	return nil
}
