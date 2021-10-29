# HTTP Punching Ball

Lightweight HTTP server for testing of HTTP clients. This service supports both HTTP and HTTPS.

Execute with the flag `-h` to list the configuration options.

The source code is distributed under the [Apache License Version 2.0](./LICENSE).

## Install the service locally

```
go get -v -t -d ./...
go build -v ./...
go install -v ./...
```

## Build the docker image

```
docker build . -t aerisconsulting/http-punching-ball && docker push aerisconsulting/http-punching-ball
```

## Use from docker

To pull and see the configuration options, run the following:

```
> docker pull aerisconsulting/http-punching-ball && docker run -it --rm aerisconsulting/http-punching-ball -h

HTTP Punching Ball is a lightweight service developed by AERIS-Consulting e.U., in order to to test HTTP clients.
The endpoint / only supports GET, POST and PUT and returns the received payload binary wrapped into a JSON body.
The endpoint /stats provides statistics about the received requests, which can be reset with a DELETE request to the same endpoint.

Usage:
  http-punching-ball [flags]

Flags:
      --debug             enables the debug mode with more verbosity
  -h, --help              help for http-punching-ball
      --http              enables the plain HTTP server (default true)
      --https             enables the HTTPS server
      --plain-port int    port for plain HTTP (default 8080)
      --ssl-cert string   certificate file for the server
      --ssl-key string    key file for the server certificate
      --ssl-port int      port for HTTPS (default 8443)

```
