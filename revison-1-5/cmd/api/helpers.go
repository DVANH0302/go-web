package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type envelope map[string]any

func (app *application) writeJson(w http.ResponseWriter, status int, body envelope) error {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)

	return encoder.Encode(body)
}

func (app *application) decodeJson(r *http.Request, dst *any) error {
	decoder := json.NewDecoder(r.Body)

	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)

	switch {
	case errors.As(err, &json.UnmarshalTypeError{}):
		return fmt.Errorf("Wrong value type")

	case errors.As(err, &json.UnsupportedTypeError{}):
		return fmt.Errorf("Wrong json format")

	case errors.Is(err, io.EOF):
		return fmt.Errorf("Empty json")
	}

	return nil

}
