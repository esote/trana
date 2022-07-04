package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/esote/trana"
	"github.com/esote/trana/internal/config"
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

type LetterDiff struct {
	R  string
	Ok bool
}

type Page struct {
	Title string
	Small bool
	Home  bool
}

type ListPage struct {
	Page
	Cards []trana.Card
}

type AddPage struct {
	Page
}

type PracticePage struct {
	Page
	Mode PracticeMode
	Card *trana.Card
}

type PracticeCheckPage struct {
	Page
	Card *trana.Card
	Ok   bool
	Diff []LetterDiff
	Mode PracticeMode
}

type DeletePage struct {
	Page
	Card *trana.Card
}

func main() {
	var dir string
	flag.StringVar(&dir, "d", "", "alternative config directory")
	flag.Parse()

	if dir == "" {
		var err error
		dir, err = config.Dir("trana")
		if err != nil {
			log.Fatal(err)
		}

	}
	file := filepath.Join(dir, "trana.db")

	deck, err := trana.NewDeck(file)
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()

	templates, err := loadTemplates()
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		cards, err := deck.List(r.Context())
		if err != nil {
			log.Fatal(err)
		}

		page := ListPage{
			Page: Page{
				Title: "Cards",
				Home:  true,
			},
			Cards: cards,
		}

		if err = templates["list.html"].ExecuteTemplate(w, "layout", &page); err != nil {
			log.Fatal(err)
		}
	})
	r.Get("/practice", func(w http.ResponseWriter, r *http.Request) {
		card, err := deck.Next(r.Context())
		if err != nil {
			log.Fatal(err)
		}

		mode := getMode(r.URL.Query())
		if mode.Reverse || (mode.Random && rand.Intn(2) == 0) {
			mode.Swapped = true
			card.Front, card.Back = card.Back, card.Front
		}

		page := PracticePage{
			Page: Page{
				Title: "Practice",
				Small: true,
			},
			Mode: mode,
			Card: card,
		}
		if err = templates["practice.html"].ExecuteTemplate(w, "layout", &page); err != nil {
			log.Fatal(err)
		}
	})
	r.Get("/practice_check", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		id, err := strconv.ParseInt(query.Get("id"), 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		back := query.Get("back")

		card, err := deck.Get(r.Context(), id)
		if err != nil {
			log.Fatal(err)
		}

		mode := getMode(r.URL.Query())
		if mode.Swapped {
			card.Front, card.Back = card.Back, card.Front
		}

		rb := []rune(back)
		rcb := []rune(card.Back)
		for len(rb) < len(rcb) {
			rb = append(rb, '\u2007') // figure space
		}

		page := PracticeCheckPage{
			Page: Page{
				Title: "Practice",
				Small: true,
			},
			Card: card,
			Ok:   strings.EqualFold(back, card.Back),
			Mode: mode,
		}

		for i := range rb {
			page.Diff = append(page.Diff, LetterDiff{
				R:  string(rb[i]),
				Ok: i < len(rcb) && strings.EqualFold(string(rb[i]), string(rcb[i])),
			})
		}

		if err = templates["practice_check.html"].ExecuteTemplate(w, "layout", &page); err != nil {
			log.Fatal(err)
		}
	})
	r.Post("/practice_comfort", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Fatal(err)
		}

		id, err := strconv.ParseInt(r.Form.Get("id"), 10, 64)
		if err != nil {
			log.Fatal(err)
		}

		comfort, err := strconv.ParseFloat(r.Form.Get("comfort"), 64)
		if err != nil {
			log.Fatal(err)
		}

		if err = deck.Update(r.Context(), id, comfort); err != nil {
			log.Fatal(err)
		}

		mode := getMode(r.Form)
		url := url.URL{
			Path: "/practice",
		}
		query := url.Query()
		if mode.Reverse {
			query.Add("reverse", "true")
		}
		if mode.Random {
			query.Add("random", "true")
		}
		url.RawQuery = query.Encode()

		http.Redirect(w, r, url.String(), http.StatusSeeOther)
	})
	r.Get("/add", func(w http.ResponseWriter, r *http.Request) {
		page := AddPage{
			Page: Page{
				Title: "Add card",
				Small: true,
			},
		}
		if err = templates["add.html"].ExecuteTemplate(w, "layout", &page); err != nil {
			log.Fatal(err)
		}
	})
	r.Post("/add", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Fatal(err)
		}

		front := r.Form.Get("front")
		back := r.Form.Get("back")

		if err = deck.Add(r.Context(), front, back); err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	r.Get("/delete", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		card, err := deck.Get(r.Context(), id)
		if err != nil {
			log.Fatal(err)
		}

		page := DeletePage{
			Page: Page{
				Title: "Delete card",
				Small: true,
			},
			Card: card,
		}

		if err = templates["delete.html"].ExecuteTemplate(w, "layout", &page); err != nil {
			log.Fatal(err)
		}
	})
	r.Post("/delete", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Fatal(err)
		}
		id, err := strconv.ParseInt(r.Form.Get("id"), 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		if err = deck.Del(r.Context(), id); err != nil {
			log.Fatal(err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	r.Get("/export", func(w http.ResponseWriter, r *http.Request) {
		cards, err := deck.List(r.Context())
		if err != nil {
			log.Fatal(err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="trana-export-%v.json"`, now))
		w.Header().Set("Content-Type", "application/json")

		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		if err = e.Encode(cards); err != nil {
			log.Fatal(err)
		}
	})
	r.Post("/import", func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		var cards []trana.Card
		if json.NewDecoder(file).Decode(&cards); err != nil {
			log.Fatal(err)
		}

		if err = deck.Import(r.Context(), cards); err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	if err = http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
