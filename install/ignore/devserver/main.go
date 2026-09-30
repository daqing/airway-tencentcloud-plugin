// Command devserver serves the plugin's HTTP API without an Airway host
// application, so the actions can be exercised locally with curl.
//
//	go run ./install/ignore/devserver
//
// It registers every endpoint unconditionally. They are unauthenticated, so it
// listens on 127.0.0.1:3000 by default; override with LISTEN.
//
// Storage is an in-memory SQLite database — this stands in for the host's
// migrations, which a real host runs itself. Set DB_DSN to serve from a real
// PostgreSQL or MySQL instead, which also applies the module's migrations to it.
// The smsverify endpoints read the TENCENTCLOUD_* / SMS_DRIVER environment
// variables on use.
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin"
	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
)

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = "127.0.0.1:3000"
	}

	if err := setupDB(); err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	tencentcloudplugin.SmsVerifyPlugin{}.Routes(r.Group("/api/v1"))

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func setupDB() error {
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		_, err := testdb.SetupOn(dsn)
		return err
	}

	_, err := testdb.Setup()
	return err
}
