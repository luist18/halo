package main

import (
	"log"

	httpproxy "github.com/luist18/halo/proxy/http"
)

func main() {
	// TODO(PER-12): use port and endpoint as config
	proxy := httpproxy.NewHttpProxy(8080, "/sql")

	if err := proxy.Start(); err != nil {
		log.Fatal(err)
	}
}
