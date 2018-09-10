package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/asaskevich/govalidator"
	"github.com/julienschmidt/httprouter"
)

// MediaFileInfo returned to client for continuation info
type MediaFileInfo struct {
	UID  string `json:"uid"`
	Size int64  `json:"size"`
}

// NotFound object not found
var NotFound error

func handleMediaUpload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	uid := ps.ByName("uid")

	// validate parameters
	if !govalidator.IsUUID(uid) {
		w.WriteHeader(400)
		return
	}

	// check if upload is closed
	uploadable, err := isMediaFileUplodable(uid)
	if err != nil {
		log.Println("Error opening file", err)
		w.WriteHeader(500)
		return
	}

	if !uploadable {
		logNetPrintf(r, "Can not upload media %s\n", uid)
		w.WriteHeader(403)
		return
	}

	out, err := os.OpenFile(path.Join(Config.BaseDir, uid), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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

func handleMediaInfo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	uid := ps.ByName("uid")

	// validate parameters
	if !govalidator.IsUUID(uid) {
		w.WriteHeader(400)
		return
	}

	// check if registered first
	_, err := getMediaFile(uid)
	if err != nil {
		if err == NotFound {
			w.WriteHeader(404)
			return
		}
		log.Println("Error on getMediaFile", err)
		w.WriteHeader(500)
		return
	}

	var fileSize int64

	stat, err := os.Stat(path.Join(Config.BaseDir, uid))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("Error on file stat", err)
			w.WriteHeader(500)
			return
		}
		fileSize = 0
	} else {
		fileSize = stat.Size()
	}

	fileInfo := &MediaFileInfo{
		UID:  uid,
		Size: fileSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfo)
}

func handleMediaDone(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	uid := ps.ByName("uid")

	// validate parameters
	if !govalidator.IsUUID(uid) {
		w.WriteHeader(400)
		return
	}

	result, err := DB.Exec(`UPDATE media_file SET state = ? WHERE uid = ?`, 30, uid)
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

func isMediaFileUplodable(uid string) (bool, error) {
	mediaFile, err := getMediaFile(uid)
	if err != nil {
		log.Println(err)
		if err == NotFound {
			return false, nil
		}
		return false, err
	}

	return (mediaFile.State != 30), nil
}

func getMediaFile(uid string) (*MediaFile, error) {
	var mediaFile MediaFile

	row := DB.QueryRow(`SELECT id, uid, state FROM media_file WHERE uid = ?`, uid)
	err := row.Scan(&mediaFile.ID, &mediaFile.UID, &mediaFile.State)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NotFound
		}
		log.Println(err)
		return nil, err
	}

	return &mediaFile, nil
}

func logNetPrintf(r *http.Request, format string, v ...interface{}) {
	var addr string

	if addr = r.Header.Get("x-forwarded-for"); addr == "" {
		addr = r.RemoteAddr
	}

	log.Printf("["+addr+"] "+format, v)
}
