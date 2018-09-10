package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/asaskevich/govalidator"
	"github.com/julienschmidt/httprouter"
)

// FileInfo is returned to android client as continuation info
type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func handleUpload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")

	// validate parameters
	if !govalidator.IsUUID(name) {
		w.WriteHeader(400)
		return
	}

	// check if upload is closed
	uploaded, err := isEvidenceUploded(name)
	if err != nil {
		log.Println("Error opening file", err)
		w.WriteHeader(500)
		return
	}

	if uploaded {
		logNetPrintf(r, "Uploading uploaded evidence %s\n", name)
		w.WriteHeader(403)
		return
	}

	out, err := os.OpenFile(path.Join(Config.BaseDir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening file", err)
		w.WriteHeader(500)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, r.Body)
	if err != nil {
		log.Println("Error writing to file", err)
		w.WriteHeader(500)
		return
	}

	err = out.Sync()
	if err != nil {
		log.Println("Error syncing file", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(200) // todo bien
}

func handleInfo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")

	// validate parameters
	if !govalidator.IsUUID(name) {
		w.WriteHeader(400)
		return
	}

	stat, err := os.Stat(path.Join(Config.BaseDir, name))
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(404)
			return
		}
		log.Println("Error on file stat", err)
		w.WriteHeader(500)
		return
	}

	fileInfo := &FileInfo{
		Name: name,
		Size: stat.Size(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfo)
}

func handleDone(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")

	// validate parameters
	if !govalidator.IsUUID(name) {
		w.WriteHeader(400)
		return
	}

	result, err := DB.Exec(`UPDATE evidence SET state = ? WHERE uid = ?`, 20, name)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	ra, err := result.RowsAffected()
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	if ra == 0 {
		w.WriteHeader(404) // harvesting possible
		return
	}

	w.WriteHeader(200)
}

func isEvidenceUploded(uid string) (bool, error) {
	evidence, err := getEvidence(uid)
	if err != nil {
		if err == NotFound {
			return false, nil
		}
		return false, err
	}

	return (evidence.State == 20), nil // TODO: make const
}

/*func logNetPrintf(r *http.Request, format string, v ...interface{}) {
  var addr string

  if addr = r.Header.Get("x-forwarded-for"); addr == "" {
    addr = r.RemoteAddr;
  }

  log.Printf("[" + addr + "] " + format, v)
}*/
