package notes

type Store interface {
	Save(id int, body string)
	Get(id int) string
	Delete(id int)
}

type NoteService struct {
	Repo Store
}

func NewService(store Store) *NoteService {
	return &NoteService{Repo: store}
}

func (s *NoteService) Create(id int, body string) {
	s.Repo.Save(id, body)
}

func (s *NoteService) Get(id int) string {
	return s.Repo.Get(id)
}

func (s *NoteService) Delete(id int) {
	s.Repo.Delete(id)
}
