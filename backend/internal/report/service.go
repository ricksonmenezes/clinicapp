package report

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidDateRange = errors.New("start must be before end")
	ErrInvalidGroupBy   = errors.New("group_by must be one of: day, week, month")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RangeInput is the parsed, not-yet-validated query params every report
// endpoint accepts. GroupBy is ignored by CommissionPayouts/ServicePopularity
// (they return one row per consultant/service, not a time series).
type RangeInput struct {
	Start   *time.Time
	End     *time.Time
	GroupBy string
}

// resolveRange defaults End to now and Start to 30 days before End when
// omitted — reports are meant to be glanceable without the caller having to
// know the data's actual date bounds up front.
func resolveRange(in RangeInput) (start, end time.Time, err error) {
	end = time.Now().UTC()
	if in.End != nil {
		end = *in.End
	}
	start = end.AddDate(0, 0, -30)
	if in.Start != nil {
		start = *in.Start
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, ErrInvalidDateRange
	}
	return start, end, nil
}

func resolveGroupBy(groupBy string) (string, error) {
	switch groupBy {
	case "":
		return "day", nil
	case "day", "week", "month":
		return groupBy, nil
	default:
		return "", ErrInvalidGroupBy
	}
}

func (s *Service) Revenue(ctx context.Context, in RangeInput) ([]RevenuePoint, time.Time, time.Time, error) {
	start, end, err := resolveRange(in)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	groupBy, err := resolveGroupBy(in.GroupBy)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	points, err := s.repo.RevenueByPeriod(ctx, start, end, groupBy)
	return points, start, end, err
}

func (s *Service) CommissionPayouts(ctx context.Context, in RangeInput) ([]CommissionPayout, time.Time, time.Time, error) {
	start, end, err := resolveRange(in)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	payouts, err := s.repo.CommissionPayouts(ctx, start, end)
	return payouts, start, end, err
}

func (s *Service) ServicePopularity(ctx context.Context, in RangeInput) ([]ServicePopularity, time.Time, time.Time, error) {
	start, end, err := resolveRange(in)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	popularity, err := s.repo.ServicePopularity(ctx, start, end)
	return popularity, start, end, err
}

func (s *Service) BookingVolume(ctx context.Context, in RangeInput) ([]BookingVolumePoint, time.Time, time.Time, error) {
	start, end, err := resolveRange(in)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	groupBy, err := resolveGroupBy(in.GroupBy)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	points, err := s.repo.BookingVolume(ctx, start, end, groupBy)
	return points, start, end, err
}
