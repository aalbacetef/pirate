package main

import (
	"flag"
	"fmt"
	"log"
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

	srv, err := pirate.NewServer(cfg)
	if err != nil {
		log.Println("error:", err)
		return
	}

	r := chi.NewRouter()
	r.Post("/*", srv.HandleRequest)

	if err := http.ListenAndServe("localhost:3939", r); err != nil {
		fmt.Println("error ListenAndServe: ", err)
	}
}
