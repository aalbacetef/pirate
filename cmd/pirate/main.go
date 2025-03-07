package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aalbacetef/pirate"
)

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

	fmt.Println("running server")

	srv, err := pirate.NewServer(cfg)
	if err != nil {
		fmt.Println("error:", err)

		if srv != nil {
			srv.Close()
		}

		return
	}

	defer srv.Close()

	r := chi.NewRouter()
	r.Post("/*", srv.HandleRequest)

	if err := http.ListenAndServe("localhost:3939", r); err != nil {
		fmt.Println("error ListenAndServe: ", err)
	}
}
