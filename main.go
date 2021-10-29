//    Copyright 2021 AERIS-Consulting e.U.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"http-punching-ball/handlers"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	configuration struct {
		debug *bool

		enablePlain *bool
		plainPort   *int

		enableSsl *bool
		sslPort   *int
		sslKey    *string
		sslCert   *string
	}
)

func init() {
	configuration.debug = rootCmd.PersistentFlags().Bool("debug", false, "enables the debug mode with more verbosity")

	configuration.plainPort = rootCmd.PersistentFlags().Int("plain-port", 8080, "port for plain HTTP")
	configuration.enablePlain = rootCmd.PersistentFlags().Bool("http", true, "enables the plain HTTP server")

	configuration.enableSsl = rootCmd.PersistentFlags().Bool("https", false, "enables the HTTPS server")
	configuration.sslPort = rootCmd.PersistentFlags().Int("ssl-port", 8443, "port for HTTPS")
	configuration.sslKey = rootCmd.PersistentFlags().String("ssl-key", "", "key file for the server certificate")
	configuration.sslCert = rootCmd.PersistentFlags().String("ssl-cert", "", "certificate file for the server")
}

var rootCmd = &cobra.Command{
	Use:   "http-punching-ball",
	Short: "HTTP Punching Ball is a dummy server that just echoes the received requests as binary",
	Long: `HTTP Punching Ball is a lightweight service developed by AERIS-Consulting e.U., in order to to test HTTP clients.
The endpoint / only supports GET, POST and PUT and returns the received payload binary wrapped into a JSON body.
The endpoint /stats provides statistics about the received requests, which can be reset with a DELETE request to the same endpoint.
`,

	Run: func(cmd *cobra.Command, args []string) {
		if *configuration.debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		router := setupRouter()
		plainServer := &http.Server{
			Addr:    ":" + strconv.Itoa(*configuration.plainPort),
			Handler: router,
		}

		tlsServer := &http.Server{
			Addr:    ":" + strconv.Itoa(*configuration.sslPort),
			Handler: router,
		}

		// Starts the plain HTTP server.
		go func() {
			if *configuration.enablePlain {
				log.Printf("Starting QALIPSIS listening %s for HTTP", plainServer.Addr)

				if err := plainServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("HTTP server start failed: %s\n", err)
				}
			}
		}()

		// Starts the HTTPS server.
		go func() {
			if *configuration.enableSsl {
				log.Printf("Starting QALIPSIS listening %s for HTTPS", tlsServer.Addr)

				if err := tlsServer.ListenAndServeTLS(*configuration.sslCert, *configuration.sslKey); err != nil && err != http.ErrServerClosed {
					log.Fatalf("HTTPS server start failed: %s\n", err)
				}
			}
		}()

		// Waiting for a signal to stop the server.
		quit := make(chan os.Signal)

		// SIGKILL can't be caught and is therefore ignored.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		// Wait for interrupt signal to gracefully shutdown the server with
		// a timeout of 5 seconds.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if *configuration.enablePlain {
			log.Println("Shutting down the HTTP server...")
			if err := plainServer.Shutdown(ctx); err != nil {
				log.Printf("HTTP Server shutdown failed: %s\n", err)
			}
		}
		if *configuration.enableSsl {
			log.Println("Shutting down the HTTPS server...")
			if err := tlsServer.Shutdown(ctx); err != nil {
				log.Printf("HTTPS Server shutdown failed: %s\n", err)
			}
		}
		// Waiting for the timeout.
		select {
		case <-ctx.Done():
		}
	},
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/", handlers.Home)
	r.POST("/", handlers.Home)
	r.PUT("/", handlers.Home)

	r.GET("/stats", handlers.RequestsStats)
	r.DELETE("/stats", handlers.ResetStats)

	return r
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
