package trana

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

const (
	ComfortReviewMin = 1
	ComfortReviewMax = 3

	ComfortMin    = ComfortReviewMin - 1
	ComfortMax    = ComfortReviewMax + 1
	ComfortStddev = 0.25
)

var ErrBadComfort = fmt.Errorf("trana: comfort must be from %d to %d", ComfortReviewMax, ComfortReviewMax)

type Trana struct {
	db *sql.DB
}

type Deck struct {
	ID   int64
	Name string
}

type Card struct {
	ID            int64
	Deck          int64
	Front         string
	Back          string
	LastPracticed *time.Time
	Comfort       float64
}

func New(path string) (*Trana, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	return &Trana{db}, nil
}

func (t *Trana) Close() error {
	return t.db.Close()
}

func (t *Trana) CreateDeck(ctx context.Context, name string) error {
	name = cleanString(name)

	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO "decks" ("name")
			VALUES (@name)`, name)
		return err
	})
}

func (t *Trana) GetDeck(ctx context.Context, id int64) (*Deck, error) {
	var deck Deck
	err := tx(t.db, ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT "id", "name"
			FROM "decks"
			WHERE "id" = @id
			LIMIT 1`, id).Scan(&deck.ID, &deck.Name)
	})
	if err != nil {
		return nil, err
	}
	return &deck, nil
}

func (t *Trana) UpdateDeck(ctx context.Context, deck *Deck) error {
	name := cleanString(deck.Name)

	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE "decks"
			SET "name" = @name
			WHERE "id" = @id`, name, deck.ID)
		return err
	})
}

func (t *Trana) DeleteDeck(ctx context.Context, id int64) error {
	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM "decks"
			WHERE "id" = @id`, id)
		return err
	})
}

func (t *Trana) ListDecks(ctx context.Context) ([]Deck, error) {
	var decks []Deck
	err := tx(t.db, ctx, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT "id", "name"
			FROM "decks"
			ORDER BY "id" ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var deck Deck
			if err = rows.Scan(&deck.ID, &deck.Name); err != nil {
				return err
			}
			decks = append(decks, deck)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return decks, nil
}

func (t *Trana) CreateCard(ctx context.Context, deck int64, front, back string) error {
	front = cleanString(front)
	back = cleanString(back)

	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO "cards" ("deck", "front", "back")
			VALUES (@deck, @front, @back)`, deck, front, back)
		return err
	})
}

func (t *Trana) GetCard(ctx context.Context, id int64) (*Card, error) {
	var card Card
	var lastPracticed sql.NullInt64

	err := tx(t.db, ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT "id", "deck", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			WHERE "id" = @id
			LIMIT 1`, id).Scan(&card.ID, &card.Deck, &card.Front, &card.Back, &lastPracticed, &card.Comfort)
	})
	if err != nil {
		return nil, err
	}

	if lastPracticed.Valid {
		t := time.Unix(lastPracticed.Int64, 0)
		card.LastPracticed = &t
	}
	return &card, nil
}

func (t *Trana) NextCard(ctx context.Context, deck int64) (*Card, error) {
	var card Card
	var lastPracticed sql.NullInt64

	err := tx(t.db, ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT "id", "deck", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			WHERE "deck" = @deck
			ORDER BY "comfort" ASC, RANDOM()
			LIMIT 1`, deck).Scan(&card.ID, &card.Deck, &card.Front, &card.Back, &lastPracticed, &card.Comfort)
	})
	if err != nil {
		return nil, err
	}

	if lastPracticed.Valid {
		t := time.Unix(lastPracticed.Int64, 0)
		card.LastPracticed = &t
	}
	return &card, nil
}

func (t *Trana) UpdateCard(ctx context.Context, card *Card) error {
	if card == nil {
		return errors.New("card is nil")
	}
	if card.Comfort != -1 && (card.Comfort < ComfortMin || card.Comfort > ComfortMax) {
		return ErrBadComfort
	}

	front := cleanString(card.Front)
	back := cleanString(card.Back)
	var lastPracticed sql.NullInt64
	if card.LastPracticed != nil {
		lastPracticed.Valid = true
		lastPracticed.Int64 = card.LastPracticed.Unix()
	}

	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE "cards"
			SET "front" = @front, "back" = @back, "last_practiced" = @lastPracticed, "comfort" = @comfort
			WHERE "id" = @id`, front, back, lastPracticed, card.Comfort, card.ID)
		return err
	})
}

func (t *Trana) ReviewCard(ctx context.Context, id int64, comfort float64) error {
	if comfort < ComfortReviewMin || comfort > ComfortReviewMax {
		return ErrBadComfort
	}
	comfort = comfortNorm(comfort)

	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE "cards"
			SET "comfort" = @comfort
			WHERE "id" = @id`, comfort, id)
		return err
	})
}

func (t *Trana) DeleteCard(ctx context.Context, id int64) error {
	return tx(t.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM "cards"
			WHERE "id" = @id`, id)
		return err
	})
}

func (t *Trana) ListCards(ctx context.Context, deck int64) ([]Card, error) {
	var cards []Card
	err := tx(t.db, ctx, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT "id", "deck", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			WHERE "deck" = @deck
			ORDER BY "id" ASC`, deck)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var card Card
			var lastPracticed sql.NullInt64
			if err = rows.Scan(&card.ID, &card.Deck, &card.Front, &card.Back, &lastPracticed, &card.Comfort); err != nil {
				return err
			}
			if lastPracticed.Valid {
				t := time.Unix(lastPracticed.Int64, 0)
				card.LastPracticed = &t
			}
			cards = append(cards, card)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

func (t *Trana) Import(ctx context.Context, deck int64, cards []Card) error {
	return tx(t.db, ctx, func(tx *sql.Tx) error {
		for _, card := range cards {
			if err := importCard(tx, deck, &card); err != nil {
				return err
			}
		}
		return nil
	})
}

func importCard(tx *sql.Tx, deck int64, card *Card) error {
	var lastPracticed sql.NullInt64
	if card.LastPracticed != nil {
		lastPracticed.Valid = true
		lastPracticed.Int64 = card.LastPracticed.Unix()
	}

	card.Front = cleanString(card.Front)
	card.Back = cleanString(card.Back)

	var id int64
	var back string
	err := tx.QueryRow(`SELECT "id", "back"
		FROM "cards"
		WHERE "deck" = @deck AND "front" = @front
		LIMIT 1`, deck, card.Front).Scan(&id, &back)
	if err == nil {
		if back == card.Back {
			// Card is already in deck, override last_practiced and comfort
			_, err = tx.Exec(`UPDATE "cards"
				SET "last_practiced" = @lastPracticed, "comfort" = @comfort
				WHERE "id" = @id`, lastPracticed, card.Comfort, id)
			return err
		}
		return fmt.Errorf("imported card %d duplicates existing card %d (same front, different back)", card.ID, id)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = tx.Exec(`INSERT INTO "cards" ("deck", "front", "back", "last_practiced", "comfort")
		VALUES (@deck, @front, @back, @lastPracticed, @comfort)`, deck, card.Front, card.Back, lastPracticed, card.Comfort)
	return err
}

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = norm.NFC.String(s)
	return s
}

func comfortNorm(comfort float64) float64 {
	return truncNorm(ComfortMin, ComfortMax, comfort, ComfortStddev)
}

func truncNorm(min, max, mean, stddev float64) float64 {
	if min >= max {
		panic(fmt.Sprintf("min %f >= max %f", min, max))
	}
	for {
		x := rand.NormFloat64()*stddev + mean
		if x >= min && x < max {
			return x
		}
	}
}
