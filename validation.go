package main

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/asaskevich/govalidator"
)

func init() {
	var (
		fileExts = map[string]bool{
			"jpg":   true,
			"jpeg":  true,
			"aac":   true,
			"mp4":   true,
			"mpeg4": true,
		}

		rxWhistlerCells = regexp.MustCompile("^[0-9a-zA-Z:, -]+$")
	)

	govalidator.TagMap["whistlerfile"] = govalidator.Validator(func(str string) bool {
		if govalidator.IsNull(str) {
			return true
		}

		parts := strings.Split(str, ".")
		if len(parts) != 2 {
			return false
		}

		return govalidator.IsUUID(parts[0]) && fileExts[strings.ToLower(parts[1])]
	})

	govalidator.TagMap["whistlerfileext"] = govalidator.Validator(func(str string) bool {
		if govalidator.IsNull(str) {
			return true
		}

		if len(str) < 2 { // at least ".x"
			return false
		}

		return fileExts[strings.ToLower(str[1:])]
	})

	govalidator.TagMap["whistlercells"] = govalidator.Validator(func(str string) bool {
		if govalidator.IsNull(str) {
			return true
		}
		return rxWhistlerCells.MatchString(str)
	})
}

func validateStruct(w http.ResponseWriter, name string, s interface{}) bool {
	vres, err := govalidator.ValidateStruct(s)
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(400)
		return false
	}

	if !vres {
		log.Printf("ValidateStruct for %s returned false\n", name)
		w.WriteHeader(400)
		return false
	}

	return true
}

func validateMetadata(w http.ResponseWriter, name string, m Metadata) bool {
	if !validateStruct(w, name, m) {
		return false
	}

	for _, cell := range m.Cells {
		if !govalidator.TagMap["whistlercells"](cell) {
			log.Printf("%s does not validate as whistlercells\n", cell)
			w.WriteHeader(400)
			return false
		}
	}

	return true
}

func validateMediafile(w http.ResponseWriter, name string, mediaFile MediaFile) bool {
	if !validateStruct(w, name, mediaFile) {
		return false
	}

	if !validateMetadata(w, name+".Metadata", mediaFile.Metadata) {
		return false
	}

	return true
}
