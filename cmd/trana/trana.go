package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/esote/configdir"
	"github.com/esote/trana"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed templates/*
var templateFiles embed.FS

func loadTemplates() (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)
	files, err := fs.ReadDir(templateFiles, "templates")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		t, err := template.ParseFS(templateFiles, filepath.Join("templates", f.Name()), filepath.Join("templates", "layout.html"))
		if err != nil {
			return nil, err
		}
		templates[f.Name()] = t
	}
	return templates, nil
}

func main() {
	var dir string
	flag.StringVar(&dir, "d", "", "alternative config directory")
	flag.Parse()

	if dir == "" {
		var err error
		dir, err = configdir.New("trana")
		if err != nil {
			log.Fatal(err)
		}

	}
	file := filepath.Join(dir, "trana.db")

	deck, err := trana.New(file)
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()

	templates, err := loadTemplates()
	if err != nil {
		log.Fatal(err)
	}

	server := server{deck, templates}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", server.ListDecks)

	r.Get("/deck/create", server.CreateDeck)
	r.Post("/deck/create", server.CreateDeckSubmit)

	r.Get("/deck/update", server.UpdateDeck)
	r.Post("/deck/update", server.UpdateDeckSubmit)

	r.Get("/deck/delete", server.DeleteDeck)
	r.Post("/deck/delete", server.DeleteDeckSubmit)

	r.Get("/cards", server.ListCards)

	r.Get("/card/create", server.CreateCard)
	r.Post("/card/create", server.CreateCardSubmit)

	r.Get("/card/practice", server.PracticeCard)
	r.Get("/card/check", server.CheckCard)
	r.Post("/card/check", server.CheckCardSubmit)

	r.Get("/card/update", server.UpdateCard)
	r.Post("/card/update", server.UpdateCardSubmit)

	r.Get("/card/delete", server.DeleteCard)
	r.Post("/card/delete", server.DeleteCardSubmit)

	r.Post("/import", server.ImportCards)
	r.Get("/export", server.ExportCards)

	if err = http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	trana     *trana.Trana
	templates map[string]*template.Template
}

func (s *server) template(name string, w io.Writer, page any) error {
	return s.templates[name+".html"].ExecuteTemplate(w, "layout", page)
}

type ListDecks struct {
	Decks []trana.Deck
}

func (s *server) CreateDeck(w http.ResponseWriter, r *http.Request) {
	if err := s.template("deck_create", w, nil); err != nil {
		log.Fatal(err)
	}
}

func (s *server) CreateDeckSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	name := r.Form.Get("name")

	if err := s.trana.CreateDeck(r.Context(), name); err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type UpdateDeck struct {
	Deck *trana.Deck
}

func (s *server) UpdateDeck(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page UpdateDeck

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("deck_update", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) UpdateDeckSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	var err error
	var deck trana.Deck

	deck.ID, err = strconv.ParseInt(r.Form.Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	deck.Name = r.Form.Get("name")

	if err = s.trana.UpdateDeck(r.Context(), &deck); err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type DeleteDeck struct {
	Card *trana.Card
}

func (s *server) DeleteDeck(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page DeleteCard

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("deck_delete", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) DeleteDeckSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	deck, err := strconv.ParseInt(r.Form.Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.trana.DeleteDeck(r.Context(), deck); err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) ListDecks(w http.ResponseWriter, r *http.Request) {
	var err error
	var page ListDecks

	page.Decks, err = s.trana.ListDecks(r.Context())
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("decks", w, &page); err != nil {
		log.Fatal(err)
	}
}

type CreateCard struct {
	Deck *trana.Deck
}

func (s *server) CreateCard(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page CreateCard

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("card_create", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) CreateCardSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	deck, err := strconv.ParseInt(r.Form.Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	front := r.Form.Get("front")
	back := r.Form.Get("back")

	if err = s.trana.CreateCard(r.Context(), deck, front, back); err != nil {
		log.Fatal(err)
	}

	url := url.URL{
		Path: "/cards",
	}
	query := url.Query()
	query.Add("deck", r.Form.Get("deck"))
	url.RawQuery = query.Encode()

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

type PracticeMode struct {
	// Current card is swapped
	Swapped bool

	// General behavior
	Reverse bool
	Random  bool
}

func getMode(v url.Values) PracticeMode {
	return PracticeMode{
		Swapped: v.Get("swapped") == "true",
		Reverse: v.Get("reverse") == "true",
		Random:  v.Get("random") == "true",
	}
}

type PracticeCard struct {
	Deck *trana.Deck
	Card *trana.Card
	Mode PracticeMode
}

func (s *server) PracticeCard(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page PracticeCard

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Card, err = s.trana.NextCard(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Mode = getMode(r.URL.Query())
	if page.Mode.Reverse || (page.Mode.Random && rand.Intn(2) == 0) {
		page.Mode.Swapped = true
		page.Card.Front, page.Card.Back = page.Card.Back, page.Card.Front
	}

	if err = s.template("card_practice", w, &page); err != nil {
		log.Fatal(err)
	}
}

type LetterDiff struct {
	R  string
	Ok bool
}

func unicodeDiff(got, want string, fill rune) []LetterDiff {
	var diff []LetterDiff
	gotRunes := []rune(got)
	wantRunes := []rune(want)

	for len(gotRunes) < len(wantRunes) {
		gotRunes = append(gotRunes, fill)
	}

	for i := range gotRunes {
		diff = append(diff, LetterDiff{
			R:  string(gotRunes[i]),
			Ok: i < len(wantRunes) && strings.EqualFold(string(gotRunes[i]), string(wantRunes[i])),
		})
	}
	return diff
}

type CheckCard struct {
	Deck *trana.Deck
	Card *trana.Card
	Mode PracticeMode

	Ok   bool
	Diff []LetterDiff
}

func (s *server) CheckCard(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	card, err := strconv.ParseInt(r.URL.Query().Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	back := r.URL.Query().Get("back")

	var page CheckCard

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Card, err = s.trana.GetCard(r.Context(), card)
	if err != nil {
		log.Fatal(err)
	}

	page.Mode = getMode(r.URL.Query())
	if page.Mode.Swapped {
		page.Card.Front, page.Card.Back = page.Card.Back, page.Card.Front
	}

	page.Ok = strings.EqualFold(back, page.Card.Back)

	const figureSpace = '\u2007'
	page.Diff = unicodeDiff(back, page.Card.Back, figureSpace)

	if err = s.template("card_check", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) CheckCardSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	deck := r.Form.Get("deck")

	card, err := strconv.ParseInt(r.Form.Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	comfort, err := strconv.ParseFloat(r.Form.Get("comfort"), 64)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.trana.ReviewCard(r.Context(), card, comfort); err != nil {
		log.Fatal(err)
	}

	mode := getMode(r.Form)
	url := url.URL{
		Path: "/card/practice",
	}
	query := url.Query()
	query.Add("deck", deck)
	if mode.Reverse {
		query.Add("reverse", "true")
	}
	if mode.Random {
		query.Add("random", "true")
	}
	url.RawQuery = query.Encode()

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

type UpdateCard struct {
	Deck *trana.Deck
	Card *trana.Card

	TimeFormat             string
	ComfortMin, ComfortMax float64
}

const dateTimeLocal = "2006-01-02T15:04"

func (s *server) UpdateCard(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	card, err := strconv.ParseInt(r.URL.Query().Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	page := UpdateCard{
		TimeFormat: dateTimeLocal,
		ComfortMin: trana.ComfortMin,
		ComfortMax: trana.ComfortMax,
	}

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Card, err = s.trana.GetCard(r.Context(), card)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("card_update", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) UpdateCardSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	var err error
	var card trana.Card

	card.ID, err = strconv.ParseInt(r.Form.Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	card.Front = r.Form.Get("front")
	card.Back = r.Form.Get("back")

	if r.Form.Get("last_practiced") != "" {
		t, err := time.ParseInLocation(dateTimeLocal, r.Form.Get("last_practiced"), time.Local)
		if err != nil {
			log.Fatal(err)
		}
		card.LastPracticed = &t
	}

	comfort := r.Form.Get("comfort")
	if comfort == "" {
		card.Comfort = -1
	} else {
		card.Comfort, err = strconv.ParseFloat(comfort, 64)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err = s.trana.UpdateCard(r.Context(), &card); err != nil {
		log.Fatal(err)
	}

	url := url.URL{
		Path: "/cards",
	}
	query := url.Query()
	query.Add("deck", r.Form.Get("deck"))
	url.RawQuery = query.Encode()

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

type DeleteCard struct {
	Deck *trana.Deck
	Card *trana.Card
}

func (s *server) DeleteCard(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	card, err := strconv.ParseInt(r.URL.Query().Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page DeleteCard

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Card, err = s.trana.GetCard(r.Context(), card)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("card_delete", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) DeleteCardSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal(err)
	}

	card, err := strconv.ParseInt(r.Form.Get("card"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.trana.DeleteCard(r.Context(), card); err != nil {
		log.Fatal(err)
	}

	url := url.URL{
		Path: "/cards",
	}
	query := url.Query()
	query.Add("deck", r.Form.Get("deck"))
	url.RawQuery = query.Encode()

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

type ListCards struct {
	Deck  *trana.Deck
	Cards []trana.Card
}

func (s *server) ListCards(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var page ListCards

	page.Deck, err = s.trana.GetDeck(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	page.Cards, err = s.trana.ListCards(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}

	if err = s.template("cards", w, &page); err != nil {
		log.Fatal(err)
	}
}

func (s *server) ImportCards(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	deck, err := strconv.ParseInt(r.Form.Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	var cards []trana.Card
	if json.NewDecoder(file).Decode(&cards); err != nil {
		log.Fatal(err)
	}

	if err = s.trana.Import(r.Context(), deck, cards); err != nil {
		log.Fatal(err)
	}

	url := url.URL{
		Path: "/cards",
	}
	query := url.Query()
	query.Add("deck", r.Form.Get("deck"))
	url.RawQuery = query.Encode()

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

func (s *server) ExportCards(w http.ResponseWriter, r *http.Request) {
	deck, err := strconv.ParseInt(r.URL.Query().Get("deck"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	cards, err := s.trana.ListCards(r.Context(), deck)
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="trana-deck-%d-%s.json"`, deck, now))
	w.Header().Set("Content-Type", "application/json")

	e := json.NewEncoder(w)
	e.SetIndent("", "\t")
	if err = e.Encode(cards); err != nil {
		log.Fatal(err)
	}
}
