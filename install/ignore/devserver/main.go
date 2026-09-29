// Command devserver serves the plugin's HTTP API without an Airway host
// application, so the actions can be exercised locally with curl.
//
//	go run ./install/ignore/devserver
//
// It registers every endpoint unconditionally. They are unauthenticated, so it
// listens on 127.0.0.1:3000 by default; override with LISTEN.
//
// Storage is an in-memory SQLite database — this stands in for the host's
// migrations, which a real host runs itself. The tencentcloud debug endpoints
// need no configuration; the real /sms/send and the smsverify endpoints read
// the TENCENTCLOUD_* / SMS_DRIVER environment variables on use.
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin"
	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/api/tencentcloud_api"
)

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = "127.0.0.1:3000"
	}

	if _, err := testdb.Setup(); err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	tencentcloud_api.DebugRoutes(r.Group("/api/v1/tencentcloud"))
	tencentcloudplugin.SmsVerifyPlugin{}.Routes(r.Group("/api/v1"))

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
