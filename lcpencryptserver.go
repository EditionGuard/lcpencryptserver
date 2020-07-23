package main

import  (
"io"
"os"
"os/exec"
"net/http"
"github.com/joho/godotenv"
"github.com/gorilla/mux"
"log"
"bytes"
)

func main() {
    err := godotenv.Load()
    if err != nil {
      log.Print("No .env file, continuing.")
    }

    port, exists := os.LookupEnv("LISTEN_PORT")
    if !exists {
      log.Print("No port specified, using default 8992.")
      port = "8992"
    }

    requiredVars := []string{"LCP_SERVER_URL","LCP_SERVER_LOGIN","LCP_SERVER_PASSWORD","STORAGE_PATH","LCP_ENCRYPT_PATH"}
    for _, varName := range requiredVars {
      _, exists := os.LookupEnv(varName)
      if(!exists) {
        log.Fatal("Please set environment variable: " + varName)
      }
    }
    router := mux.NewRouter()
    router.
        Path("/upload").
        Methods("POST").
        HandlerFunc(UploadFile)
    log.Print("Starting LCP Encryption Server on port " + port)
    log.Fatal(http.ListenAndServe(":" + port, router))
}

func UploadFile(w http.ResponseWriter, r *http.Request) {
    err := r.ParseMultipartForm(5 * 1024 * 1024)
    w.Header().Set("Content-Type", "application/json")

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    file, header, err := r.FormFile("file")

    if file == nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if header == nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    defer file.Close()

    path := os.Getenv("STORAGE_PATH") + "/" + header.Filename
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    defer f.Close()
    io.Copy(f, file)

    // TODO: Call executable with received file, then place response into encryptResult
    lcpencrypt := os.Getenv("LCP_ENCRYPT_PATH")
    args := []string{lcpencrypt, "-input", path, "-lcpsv", os.Getenv("LCP_SERVER_URL"), "-login", os.Getenv("LCP_SERVER_LOGIN"), "-password", os.Getenv("LCP_SERVER_PASSWORD")}
    contentid := r.PostFormValue("contentid")
    if contentid == "" {
      log.Print("Creating new publication as no contentid was provided.")
    } else {
      args = append(args, "-contentid", contentid)
    }

    var buffer bytes.Buffer
    cmd := &exec.Cmd {
        Path: lcpencrypt,
        Args: args,
        Stdout: &buffer,
        Stderr: &buffer,
    }

    // run `go version` command
    if err := cmd.Run(); err != nil {
        log.Print( "Encrypt Command Execution Error:", err );
    }
    encryptResult := buffer.Bytes()
    log.Print("Response sent: " + string(encryptResult))
    w.Write(encryptResult)
}
