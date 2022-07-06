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

var (
	ErrNoCard     = errors.New("trana: no cards in deck")
	ErrBadComfort = fmt.Errorf("trana: comfort must be from %d to %d", ComfortMax, ComfortMax)
)

type Deck struct {
	db *sql.DB
}

const (
	ComfortMin = 1
	ComfortMax = 3

	ComfortNormMin = ComfortMin - 1
	ComfortNormMax = ComfortMax + 1
	ComfortStddev  = 0.25
)

type Card struct {
	ID            int64
	Front         string
	Back          string
	LastPracticed *time.Time
	Comfort       float64
}

func NewDeck(path string) (*Deck, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	return &Deck{db}, nil
}

func (d *Deck) Close() error {
	return d.db.Close()
}

func (d *Deck) Add(ctx context.Context, front, back string) error {
	front = cleanString(front)
	back = cleanString(back)

	return tx(d.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO "cards" ("front", "back")
			VALUES (@front, @back)`, front, back)
		return err
	})
}

func (d *Deck) Del(ctx context.Context, id int64) error {
	err := tx(d.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM "cards" WHERE "id" = @id`, id)
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoCard
	}
	return err
}

func (d *Deck) Get(ctx context.Context, id int64) (*Card, error) {
	var card Card
	err := tx(d.db, ctx, func(tx *sql.Tx) error {
		var lastPracticed sql.NullInt64
		err := tx.QueryRow(`SELECT "id", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			WHERE "id" = @id`, id).Scan(&card.ID, &card.Front, &card.Back, &lastPracticed, &card.Comfort)
		if err != nil {
			return err
		}
		if lastPracticed.Valid {
			t := time.Unix(lastPracticed.Int64, 0)
			card.LastPracticed = &t
		}
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoCard
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (d *Deck) Next(ctx context.Context) (*Card, error) {
	var card Card
	err := tx(d.db, ctx, func(tx *sql.Tx) error {
		var lastPracticed sql.NullInt64
		err := tx.QueryRow(`SELECT "id", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			ORDER BY "comfort" ASC, RANDOM()
			LIMIT 1`).Scan(&card.ID, &card.Front, &card.Back, &lastPracticed, &card.Comfort)
		if err != nil {
			return err
		}
		if lastPracticed.Valid {
			t := time.Unix(lastPracticed.Int64, 0)
			card.LastPracticed = &t
		}
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoCard
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (d *Deck) Update(ctx context.Context, id int64, comfort float64) error {
	if comfort < ComfortMin || comfort > ComfortMax {
		return ErrBadComfort
	}
	err := tx(d.db, ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE "cards"
			SET "last_practiced" = strftime('%s', 'now'), "comfort" = @comfort
			WHERE "id" = @id`, comfortNorm(comfort), id)
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoCard
	}
	return err
}

func (d *Deck) List(ctx context.Context) ([]Card, error) {
	var cards []Card
	err := tx(d.db, ctx, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT "id", "front", "back", "last_practiced", "comfort"
			FROM "cards"
			ORDER BY "id" ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var card Card
			var lastPracticed sql.NullInt64
			if err = rows.Scan(&card.ID, &card.Front, &card.Back, &lastPracticed, &card.Comfort); err != nil {
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

func (d *Deck) Import(ctx context.Context, cards []Card) error {
	return tx(d.db, ctx, func(tx *sql.Tx) error {
		for _, card := range cards {
			if err := importCard(tx, &card); err != nil {
				return err
			}
		}
		return nil
	})
}

func importCard(tx *sql.Tx, card *Card) error {
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
		WHERE "front" = @front
		LIMIT 1`, card.Front, card.Back).Scan(&id, &back)
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

	_, err = tx.Exec(`INSERT INTO "cards" ("front", "back", "last_practiced", "comfort")
		VALUES (@front, @back, @lastPracticed, @comfort)`, card.Front, card.Back, lastPracticed, card.Comfort)
	return err
}

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = norm.NFC.String(s)
	return s
}

func comfortNorm(comfort float64) float64 {
	return truncNorm(ComfortNormMin, ComfortNormMax, comfort, ComfortStddev)
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
