package main

import  (
"io"
"os"
"net/http"
"errors"
"bytes"
"path/filepath"
"github.com/readium/readium-lcp-server/lcpencrypt/encrypt"
"github.com/readium/readium-lcp-server/pack"
"github.com/readium/readium-lcp-server/license"
"github.com/readium/readium-lcp-server/lcpserver/api"
"github.com/joho/godotenv"
"github.com/gorilla/mux"
"fmt"
"log"
"encoding/json"
uuid "github.com/satori/go.uuid"
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

    requiredVars := []string{"LCP_SERVER_URL","LCP_SERVER_LOGIN","LCP_SERVER_PASSWORD","STORAGE_PATH"}
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
    var lcpPublication apilcp.LcpPublication
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

    contentid := r.PostFormValue("contentid")

    defer file.Close()

    path := os.Getenv("STORAGE_PATH") + "/" + header.Filename
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    defer f.Close()
    io.Copy(f, file)

    log.Print("Request received for file " + header.Filename)
    if contentid == "" {
  		uid, err := uuid.NewV4()
      if err != nil {
          http.Error(w, err.Error(), http.StatusInternalServerError)
          return
      }
  		contentid = uid.String()
      log.Print("No contentid was given, using new id " + contentid)
    }
    lcpPublication.ContentId = contentid

    var encryptionArtifact encrypt.EncryptionArtifact
    var encryptionError = errors.New("Failed")
    outputPath := os.Getenv("STORAGE_PATH") + "/" + contentid

    ext := filepath.Ext(path)
    extension := ".epub"

    // Epub file encrypted directly.
    if ext == ".epub" {
      outputPath += extension
      log.Print("Encrypting ePub with path " + outputPath)
      encryptionArtifact, encryptionError = encrypt.EncryptEpub(path, outputPath)
    // PDF File needs to be built as web pub first, then encrypted.
    } else {
      extension = ".lcpdf"
      outputPath += extension
      log.Print("Encrypting PDF with path " + outputPath)
      lcpPublication.ContentType = "application/pdf+lcp"
      err := pack.BuildWebPubPackageFromPDF(filepath.Base(path), path, path + ".webpub")
  		if err != nil {
  			http.Error(w, err.Error(), http.StatusInternalServerError)
        return
  		}
      encryptionArtifact, encryptionError = encrypt.EncryptWebPubPackage(pack.EncryptionProfile(license.BASIC_PROFILE), path + ".webpub", outputPath)
    }

    if encryptionError != nil {
        http.Error(w, encryptionError.Error(), http.StatusInternalServerError)
        return
    }

    basefilename := filepath.Base(encryptionArtifact.Path)
    lcpPublication.ContentKey = encryptionArtifact.EncryptionKey
    lcpPublication.Output = encryptionArtifact.Path
    lcpPublication.Size = &encryptionArtifact.Size
    lcpPublication.Checksum = &encryptionArtifact.Checksum
    lcpPublication.ContentDisposition = &basefilename

    resp, err := notifyLcpServer(os.Getenv("LCP_SERVER_URL"), contentid, lcpPublication, os.Getenv("LCP_SERVER_LOGIN"), os.Getenv("LCP_SERVER_PASSWORD"))
		if err != nil {
			lcpPublication.ErrorMessage = "Error notifying the License Server"
			http.Error(w, err.Error(), http.StatusInternalServerError)
      return
		} else {
			log.Print("License Server was notified with status " + resp.Status)
		}

    encryptResult, err := json.Marshal(lcpPublication)

    log.Print("Response sent: " + string(encryptResult))
    w.Write(encryptResult)
}

func notifyLcpServer(lcpService string, contentid string, lcpPublication apilcp.LcpPublication, username string, password string) (*http.Response, error) {
	//exchange encryption key with lcp service/content/<id>,
	//Payload:
	//  content-id: unique id for the content
	//  content-encryption-key: encryption key used for the content
	//  protected-content-location: full path of the encrypted file
	//  protected-content-length: content length in bytes
	//  protected-content-sha256: content sha
	//  protected-content-disposition: encrypted file name
	//  protected-content-type: encrypted file content type
	//fmt.Printf("lcpsv = %s\n", *lcpsv)
	var urlBuffer bytes.Buffer
	urlBuffer.WriteString(lcpService)
	urlBuffer.WriteString("/contents/")
	urlBuffer.WriteString(contentid)

	jsonBody, err := json.Marshal(lcpPublication)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PUT", urlBuffer.String(), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if (resp.StatusCode != 302) && (resp.StatusCode/100) != 2 { //302=found or 20x reply = OK
		return nil, fmt.Errorf("lcp server error %d", resp.StatusCode)
	}

	return resp, nil
}
