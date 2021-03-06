# lcpencryptserver
A simple Go http server alternative for the lcpencrypt tool bundled with the Readium  LCP Server.

## Installation

First install the prerequisite go modules.

go get github.com/readium/readium-lcp-server
go get github.com/joho/godotenv
go get github.com/gorilla/mux

Then build the server.

go build lcpencryptserver.go

## Usage

### Configuration

Before running the server, you must set the following environment variables (you can use a .env file in the same folder as well);

- LCP_SERVER_URL (Full URL of your LCP Server, including port number)
- LCP_SERVER_LOGIN (Login for LCP Server)
- LCP_SERVER_PASSWORD (Password for LCP Server)
- STORAGE_PATH (Path for encryption operations)
- LISTEN_PORT (optional, defaults to 8992)

Then run the server with ./lcpencrypt

### Encrypting Files

You can send a multipart form POST requests to the /upload endpoint with the following properties

- file (required, the epub or pdf file to be encrypted)
- contentid (optional, content id for existing publications)
