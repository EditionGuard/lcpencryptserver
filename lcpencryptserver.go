package main

import  (
"io"
"os"
"flag"
"net/http"
"errors"
"path/filepath"
"github.com/readium/readium-lcp-server/lcpencrypt/encrypt"
"github.com/readium/readium-lcp-server/pack"
"github.com/readium/readium-lcp-server/license"
"github.com/gorilla/mux"
"fmt"
"log"
"encoding/json"
uuid "github.com/satori/go.uuid"
)

func main() {
    port := flag.String("port", "8989", "Port number to use for http comms.")
    router := mux.NewRouter()
    router.
        Path("/upload").
        Methods("POST").
        HandlerFunc(UploadFile)
    fmt.Println("Starting LCP Encryption Server on port " + *port)
    log.Fatal(http.ListenAndServe(":" + *port, router))
}

type ResponseBody struct {
    ContentId   string
    EncryptionKey []byte
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

    contentid := r.PostFormValue("contentid")

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    defer file.Close()

    path := "/tmp/" + header.Filename
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    defer f.Close()
    io.Copy(f, file)

    fmt.Println("Request received for file " + header.Filename)
    if contentid == "" {
  		uid, err := uuid.NewV4()
      if err != nil {
          http.Error(w, err.Error(), http.StatusInternalServerError)
          return
      }
  		contentid = uid.String()
      fmt.Println("No contentid was given, using new id " + contentid)
    }

    encryptionArtifact := encrypt.EncryptionArtifact{}
    encryptionError := errors.New("Failed")

    ext := filepath.Ext(path)

    // Epub file encrypted directly.
    if ext == ".epub" {
      fmt.Println("Encrypting epub")
      encryptionArtifact, encryptionError = encrypt.EncryptEpub(path, "/tmp/" + contentid + ".epub")
    // PDF File needs to be built as web pub first, then encrypted.
    } else {
      fmt.Println("Encrypting pdf")
      err := pack.BuildWebPubPackageFromPDF(filepath.Base(path), path, path + ".webpub")
  		if err != nil {
  			http.Error(w, err.Error(), http.StatusInternalServerError)
        return
  		}
      encryptionArtifact, encryptionError = encrypt.EncryptWebPubPackage(pack.EncryptionProfile(license.BASIC_PROFILE), path + ".webpub", "/tmp/" + contentid + ".lcpdf")
    }

    if encryptionError != nil {
        http.Error(w, encryptionError.Error(), http.StatusInternalServerError)
        return
    }

    encryptResult, err := json.Marshal(ResponseBody{contentid, encryptionArtifact.EncryptionKey})

    fmt.Println("Response sent: " + string(encryptResult))
    w.Write(encryptResult)
}
