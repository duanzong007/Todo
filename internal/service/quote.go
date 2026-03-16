package service

import (
	"context"

	"todo/internal/repository"
)

type QuoteService struct {
	repo *repository.QuoteRepository
}

type Quote struct {
	Text   string
	Author string
	Source string
}

func NewQuoteService(repo *repository.QuoteRepository) *QuoteService {
	return &QuoteService{repo: repo}
}

func (s *QuoteService) Random(ctx context.Context) (Quote, error) {
	if s == nil || s.repo == nil {
		return Quote{}, nil
	}

	quote, err := s.repo.Random(ctx)
	if err != nil {
		return Quote{}, err
	}

	return Quote{
		Text:   quote.Text,
		Author: quote.Author,
		Source: quote.Source,
	}, nil
}
