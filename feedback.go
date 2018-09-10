package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/asaskevich/govalidator"
	"github.com/julienschmidt/httprouter"
	mail "gopkg.in/mail.v2"
)

// Feedback struct for sending feedback
type Feedback struct {
	Name    string `json:"name"`
	Email   string `json:"email" valid:"email"`
	Message string `json:"message" valid:"required"`
}

// Validate validates feedback data
func (f *Feedback) Validate() error {
	vres, err := govalidator.ValidateStruct(f)
	if err != nil {
		return err
	}

	if !vres {
		return errors.New("Feedback validation returned false")
	}

	return nil
}

// creates feedback email body string
func createMsg(f *Feedback) (string, error) {
	var msgTemplate = `
Name: {{.Name}}
Email: {{.Email}}
Message: {{.Message}}
`

	t, err := template.New("Message").Parse(msgTemplate)
	if err != nil {
		return "", err
	}

	var res bytes.Buffer
	err = t.Execute(&res, f)
	if err != nil {
		return "", err
	}

	return res.String(), nil
}

func handleFeedback(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	body, err := ioutil.ReadAll(r.Body)
	if failed(err, w, http.StatusInternalServerError) {
		return
	}

	// decode feedback
	feedback := &Feedback{}
	err = json.Unmarshal(body, feedback)
	if failed(err, w, http.StatusBadRequest) {
		return
	}

	// validate Feedback obj
	err = feedback.Validate()
	if failed(err, w, http.StatusBadRequest) {
		return
	}

	// create msg body
	msg, err := createMsg(feedback)
	if failed(err, w, http.StatusBadRequest) {
		return
	}

	// send email
	message := mail.NewMessage()
	message.SetHeader("From", Config.FeedbackMailTo)
	message.SetHeader("To", Config.FeedbackMailTo)
	message.SetHeader("Subject", Config.FeedbackMailSubject)
	message.SetBody("text/plain", msg)

	d := mail.NewDialer(Config.FeedbackMailSMTPHost, Config.FeedbackMailSMTPPort, "", "")
	d.LocalName = Config.FeedbackMailLocalHost
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	err = d.DialAndSend(message)
	if failed(err, w, http.StatusInternalServerError) {
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
}

func failed(err error, w http.ResponseWriter, status int) bool {
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(status)
		return true
	}

	return false
}
