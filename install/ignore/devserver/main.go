// Command devserver serves the plugin's HTTP API without an Airway host
// application, so the actions can be exercised locally with curl.
//
//	go run ./install/ignore/devserver
//
// It listens on :3000 by default; override with LISTEN. The mock endpoint
// needs no configuration; the real /sms/send reads the TENCENTCLOUD_*
// environment variables on first use.
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/api/tencentcloud_api"
)

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = ":3000"
	}

	r := gin.Default()
	tencentcloud_api.Routes(r.Group("/api/v1/tencentcloud"))

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
