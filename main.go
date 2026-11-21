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
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"http-punching-ball/handlers"
)

// version is injected at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	shutdownTimeout    = 5 * time.Second
	healthCheckTimeout = 2 * time.Second
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
	configuration.debug = rootCmd.PersistentFlags().Bool("debug", false, "enables the debug mode, logging every request")

	configuration.plainPort = rootCmd.PersistentFlags().Int("plain-port", 8080, "port for plain HTTP")
	configuration.enablePlain = rootCmd.PersistentFlags().Bool("http", true, "enables the plain HTTP server")

	configuration.enableSsl = rootCmd.PersistentFlags().Bool("https", false, "enables the HTTPS server")
	configuration.sslPort = rootCmd.PersistentFlags().Int("ssl-port", 8443, "port for HTTPS")
	configuration.sslKey = rootCmd.PersistentFlags().String("ssl-key", "http-server.key", "key file for the server certificate")
	configuration.sslCert = rootCmd.PersistentFlags().String("ssl-cert", "http-server.crt", "certificate file for the server")

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(healthCmd)
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Checks that the local plain HTTP server answers, for container health checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		client := http.Client{Timeout: healthCheckTimeout}
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/_stats", *configuration.plainPort))
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status %d", response.StatusCode)
		}
		return nil
	},
}

var rootCmd = &cobra.Command{
	Use:   "http-punching-ball",
	Short: "HTTP Punching Ball is a dummy server that just echoes the received requests as binary",
	Long: `HTTP Punching Ball is a lightweight service developed by AERIS-Consulting e.U., in order to test HTTP clients.
The endpoint / supports all the methods and returns the received payload binary wrapped into a JSON body.
The endpoint /_stats provides statistics about the received requests, which can be reset with a DELETE request to the same endpoint.
Send a request to any other URI of your choice to receive a description of your query as a JSON payload.
`,
	Version: version,

	Run: func(cmd *cobra.Command, args []string) {
		if *configuration.debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		router := setupRouter(*configuration.debug)
		plainServer := newServer(*configuration.plainPort, router)
		tlsServer := newServer(*configuration.sslPort, router)

		// Starts the plain HTTP server.
		go func() {
			if *configuration.enablePlain {
				log.Printf("Starting HTTP Punching Ball, listening on %s for HTTP", plainServer.Addr)

				if err := plainServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("HTTP server start failed: %s\n", err)
				}
			}
		}()

		// Starts the HTTPS server.
		go func() {
			if *configuration.enableSsl {
				log.Printf("Starting HTTP Punching Ball, listening on %s for HTTPS", tlsServer.Addr)

				if err := tlsServer.ListenAndServeTLS(*configuration.sslCert, *configuration.sslKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("HTTPS server start failed: %s\n", err)
				}
			}
		}()

		// Waiting for a signal to stop the server.
		quit := make(chan os.Signal, 1)

		// SIGKILL can't be caught and is therefore ignored.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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
	},
}

func newServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:        ":" + strconv.Itoa(port),
		Handler:     handler,
		ConnContext: handlers.ConnContext,
	}
}

// setupRouter creates the router without per-request logging, unless the debug mode is enabled,
// so that the server remains a cheap target for load tests.
func setupRouter(debug bool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	if debug {
		r.Use(gin.Logger())
	}

	r.Any("/", handlers.Home)
	r.GET("/_stats", handlers.RequestsStats)
	r.DELETE("/_stats", handlers.ResetStats)

	r.NoRoute(handlers.Describe)

	return r
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
