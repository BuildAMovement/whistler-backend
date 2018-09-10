package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/codingconcepts/env"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
)

// DB all package is using
var DB *sql.DB

// getEvidence gets first Evidence with given uid, all others shoud have same state.
// Others are evidences from same phone atached to different Reports.
func getEvidence(uid string) (*Evidence, error) {
	var evidence Evidence

	row := DB.QueryRow(`SELECT reportId, uid, state FROM evidence WHERE uid = ? LIMIT 1`, uid)
	err := row.Scan(&evidence.ReportID, &evidence.UID, &evidence.State)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NotFound
		}
		log.Println(err)
		return nil, err
	}

	return &evidence, nil
}

// WhistlerConfig struct defines config params
type WhistlerConfig struct {
	DataSourceName        string `env:"DATASOURCE_NAME" required:"true"`
	BaseDir               string `env:"BASE_DIR" required:"true"`
	ModuleBaseURL         string `env:"TRAIN_MODLUE_BASE_URL" required:"true"`
	FeedbackMailTo        string `env:"FM_TO" required:"true"`
	FeedbackMailSubject   string `env:"FM_SUBJECT" required:"true"`
	FeedbackMailSMTPHost  string `env:"FM_SMTP_HOST" required:"true"`
	FeedbackMailSMTPPort  int    `env:"FM_SMTP_PORT" required:"true"`
	FeedbackMailLocalHost string `env:"FM_LOCAL_HOST" required:"true"`
}

// Config holds config parameters from env
var Config WhistlerConfig

func main() {
	var err error

	Config = WhistlerConfig{}
	err = env.Set(&Config)
	if err != nil {
		log.Fatal(err)
	}

	// prepare DB
	DB, err = sql.Open("mysql", Config.DataSourceName)
	if err != nil {
		log.Fatal(err)
	}
	defer DB.Close()

	err = DB.Ping()
	if err != nil {
		log.Fatal(err)
	}

	router := httprouter.New()

	// rest
	router.POST("/rest/v1/reports", handleCreateReport)
	router.POST("/rest/v1/media/forms/registrations", handleRegisterFormMediaFiles)
	router.GET("/rest/v1/train/modules", handleListModules)
	router.POST("/rest/v1/feedback/messages", handleFeedback)
	// upload
	router.POST("/files/:name", handleUpload)
	router.POST("/files/:name/done", handleDone)
	router.GET("/files/:name/info", handleInfo)
	// media upload
	router.POST("/media/:uid", handleMediaUpload)
	router.POST("/media/:uid/done", handleMediaDone)
	router.GET("/media/:uid/info", handleMediaInfo)

	log.Fatal(http.ListenAndServe("127.0.0.1:9000", router))
}
