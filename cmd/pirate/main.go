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

	cfg, src, err := pirate.Load(cfgPath)
	if err != nil {
		fmt.Println("src: ", src)
		fmt.Println("error: ", err)
		return
	}

	srv, err := pirate.NewServer(cfg)
	if err != nil {
		fmt.Println("error:", err)

		if srv != nil {
			srv.Close()
		}

		return
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

	if err := httpSrv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("error ListenAndServe: ", err)
		}
	}
}
