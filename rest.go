package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"bytes"
	"database/sql"
	"net/url"
	"path"
	"time"

	"github.com/julienschmidt/httprouter"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// Report object being submitted by android client
type Report struct {
	ID                 int64       `json:"id,omitempty"`
	UID                string      `json:"uid,omitempty" valid:"uuid,optional"`
	Title              string      `json:"title,omitempty"`
	Location           string      `json:"location,omitempty"`
	ContactInformation string      `json:"contactInformation,omitempty"`
	Date               int64       `json:"date,omitempty"`
	Created            int64       `json:"created,omitempty"`
	Public             bool        `json:"public,omitempty"`
	Status             uint8       `json:"status,omitempty"`
	JSON               []byte      `json:"json,omitempty"`
	Evidences          []Evidence  `json:"evidences" valid:"required"`
	Recipients         []Recipient `json:"recipients" valid:"required"`
}

// Metadata submitted with MediaFile & Evidence
type Metadata struct {
	Cells              []string `json:"cells,omitempty" validate:"whistlercells"`
	Wifis              []string `json:"wifis,omitempty"`
	Timestamp          int64    `json:"timestamp,omitempty"`
	AmbientTemperature float64  `json:"ambientTemperature,omitempty"`
	Light              float64  `json:"light,omitempty"`
	Location           Location `json:"location,omitempty"`
}

// Evidence object listed in Report
type Evidence struct {
	UID      string   `json:"uid,omitempty" valid:"uuid,optional"`
	ReportID int64    `json:"reportId,omitempty"`
	Name     string   `json:"name,omitempty" valid:"uuid,optional"`
	State    int8     `json:"state,omitempty"`
	Path     string   `json:"path" valid:"whistlerfile,optional"`
	Metadata Metadata `json:"metadata,optional"`
}

// Recipient struct define Report recipient
type Recipient struct {
	Title string `json:"title,omitempty"`
	Email string `json:"email,omitempty" valid:"email,optional"`
}

// ReportResponse object returned to client
type ReportResponse struct {
	Data Report `json:"data"`
}

// Location submitted in Metadata
type Location struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Altitude  float64 `json:"altitude,omitempty"`
	Accuracy  float64 `json:"accuracy,omitempty"`
}

// MediaFile acquired by client
type MediaFile struct {
	ID       int64    `json:"id,omitempty"`
	UID      string   `json:"uid,omitempty" valid:"uuid,optional"`
	FileName string   `json:"fileName,omitempty" valid:"whistlerfile,optional"`
	FileExt  string   `json:"fileExt,omitempty" valid:"whistlerfileext,optional"`
	Metadata Metadata `json:"metadata,omitempty"`
	State    int8     `json:"state,omitempty"`
	Created  int64    `json:"created,omitempty"`
}

// FormMediaFileRegister object sent by Collect to register MediaFiles
type FormMediaFileRegister struct {
	Attachments []MediaFile `json:"attachments,omitempty"`
}

// TrainModule object describing single train module
type TrainModule struct {
	ID           int64  `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	URL          string `json:"url,omitempty"`
	Organization string `json:"organization,omitempty"`
	Type         string `json:"type,omitempty"`
	Size         int64  `json:"size,omitempty"`
}

func handleCreateReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(500)
		return
	}

	// decode report
	report := &Report{
		Public: true, // default
	}
	err = json.Unmarshal(body, report)
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(400)
		return
	}

	// validate Report struct with Evidences & Recipients
	if !validateStruct(w, "Report", report) {
		return
	}

	// validate Evidences[].Metadata (not required property)
	for _, evidence := range report.Evidences {
		if !validateMetadata(w, "Evidence.Metadata", evidence.Metadata) {
			return
		}
	}

	// set for approval only if public (0 - unreviewed, 1 - approved, 2 - rejected)
	if !report.Public {
		report.Status = 0
	}

	report = &Report{
		UID:       uuid.New().String(),
		Created:   time.Now().UTC().Unix(),
		Public:    report.Public,
		Status:    report.Status,
		Evidences: report.Evidences,
		JSON:      body,
	}

	// insert into database
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	result, err := tx.Exec(`
		INSERT INTO report (
			uid, created, public, status, json
		) VALUES (
			?, ?, ?, ?, ?
		)`, report.UID, report.Created, report.Public, report.Status, report.JSON)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	reportID, err := result.LastInsertId()
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	for _, evidence := range report.Evidences {
		// if we have evidence with this uid, than copy its state as they are same media files
		dbEvidence, err := getEvidence(evidence.Name)
		if err != nil && err != NotFound {
			log.Println(err)
			tx.Rollback()
			w.WriteHeader(500)
			return
		}

		state := int8(0)
		if dbEvidence != nil {
			state = dbEvidence.State
		}

		_, err = tx.Exec(`
			INSERT IGNORE INTO evidence (
				reportId, uid, fileExt, state 
			) VALUES (
				?, ?, ?, ?
			)`, reportID, evidence.Name, path.Ext(evidence.Path), state)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			w.WriteHeader(500)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		w.WriteHeader(500)
		return
	}

	reportResponse := &ReportResponse{
		Data: *report,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reportResponse)
}

func handleRegisterFormMediaFiles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	// decode report
	registration := &FormMediaFileRegister{}
	err = json.Unmarshal(body, registration)
	if err != nil {
		log.Println(err)
		w.WriteHeader(400)
		return
	}

	// validate FormMediaFileRegister struct with optional Metadata
	if !validateStruct(w, "FormMediaFileRegister", registration) {
		return
	}

	// validate optional Metadata
	for _, mediaFile := range registration.Attachments {
		if !validateMediafile(w, "FormMediaFileRegister.MediaFile", mediaFile) {
			return
		}
	}

	// insert into database
	for _, mediaFile := range registration.Attachments {
		mediaFile.State = 10 // todo: REGISTERED
		mediaFile.FileExt = path.Ext(mediaFile.FileName)

		metadata, err := json.Marshal(mediaFile.Metadata)
		if err != nil {
			log.Println(err)
		}

		_, err = DB.Exec(`
			INSERT INTO media_file (
				uid, fileName, fileExt, metadata, state, created
			) VALUES (
				?, ?, ?, ?, ?, ?
			)
			ON DUPLICATE KEY UPDATE 
				updated = NOW()`, mediaFile.UID, mediaFile.FileName, mediaFile.FileExt, metadata, mediaFile.State, mediaFile.Created)
		if err != nil {
			log.Println(err)
			w.WriteHeader(500)
			return
		}
	}

	w.WriteHeader(200)
}

func handleListModules(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ident := r.URL.Query().Get("ident")

	var queryBuffer bytes.Buffer

	queryBuffer.WriteString(`
		SELECT 
			train_module.id, train_module.name, train_module.path, train_organization.name, 
			train_module.type, train_module.size
		FROM
			train_module LEFT JOIN train_organization ON train_module.organizationId = train_organization.id
		WHERE
	`)

	if len(ident) > 0 {
		queryBuffer.WriteString(`train_module.ident = ?`)
	} else {
		queryBuffer.WriteString(`train_module.private = 0`)
	}

	queryBuffer.WriteString(` ORDER BY train_module.id DESC`)

	var err error
	var rows *sql.Rows

	if len(ident) > 0 {
		rows, err = DB.Query(queryBuffer.String(), ident)
	} else {
		rows, err = DB.Query(queryBuffer.String())
	}

	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	defer rows.Close()

	modules := make([]TrainModule, 0)

	for rows.Next() {
		var module TrainModule
		var moduleType sql.NullString

		err := rows.Scan(&module.ID, &module.Name, &module.URL, &module.Organization, &moduleType, &module.Size)
		if err != nil {
			log.Println(err)
			w.WriteHeader(500)
			return
		}

		if moduleType.Valid {
			module.Type = moduleType.String
		}

		url, err := url.Parse(Config.ModuleBaseURL)
		url.Path = path.Join(url.Path, module.URL)
		module.URL = url.String()

		modules = append(modules, module)
	}

	err = rows.Err()
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(modules)
}
