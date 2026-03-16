package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteRepository struct {
	db *pgxpool.Pool
}

type Quote struct {
	Text   string
	Author string
	Source string
}

func NewQuoteRepository(db *pgxpool.Pool) *QuoteRepository {
	return &QuoteRepository{db: db}
}

func (r *QuoteRepository) Random(ctx context.Context) (Quote, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			BTRIM(text) AS text,
			COALESCE(BTRIM(author), '') AS author,
			COALESCE(BTRIM(source), '') AS source
		FROM quotes
		WHERE NULLIF(BTRIM(text), '') IS NOT NULL
			AND COALESCE(LOWER(BTRIM(author)), '') <> 'duanzong'
		ORDER BY random()
		LIMIT 1
	`)

	var quote Quote
	if err := row.Scan(&quote.Text, &quote.Author, &quote.Source); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, nil
		}
		return Quote{}, fmt.Errorf("select random quote: %w", err)
	}

	quote.Text = strings.TrimSpace(quote.Text)
	quote.Author = strings.TrimSpace(quote.Author)
	quote.Source = strings.TrimSpace(quote.Source)
	return quote, nil
}
