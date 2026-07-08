package notes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type ServiceInterface interface {
	Create(id int, body string)
	Get(id int) string
	Delete(id int)
}

type NotesHandler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *NotesHandler {
	return &NotesHandler{service: service}
}

func (h *NotesHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	input := struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}{}

	json.NewDecoder(r.Body).Decode(&input)

	h.service.Create(input.ID, input.Text)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)

	json.NewEncoder(w).Encode(map[string]string{"Result": "Successfully Create!"})
}

func (h *NotesHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"Error": "Trouble parsing id"})
		return
	}

	text := h.service.Get(id)
	fmt.Println("got text", text)
	if text == "" {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"Error": "No books found"})
		return
	}

	w.WriteHeader(200)

	json.NewEncoder(w).Encode(
		map[string]struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		}{
			"Book": {id, text},
		})
}

func (h *NotesHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"Error": "Trouble parsing id"})
	}

	h.service.Delete(id)

	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"Result": "Successfully	Delete!"})

}
