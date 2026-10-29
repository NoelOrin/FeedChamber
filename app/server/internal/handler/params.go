package handler

import (
	"errors"
	"io"
	"log/slog"
	"strconv"
	"time"

	"orinforum/internal/pkg"

	"github.com/kataras/iris/v12"
)

// ListParams is the common pagination + time filter request body for list/search endpoints.
type ListParams struct {
	Page      int     `json:"page"`
	PageSize  int     `json:"page_size"`
	StartTime *string `json:"start_time"`
	EndTime   *string `json:"end_time"`
}

func (p *ListParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > pkg.PageMaxSize {
		p.PageSize = pkg.PageDefaultSize
	}
}

func (p *ListParams) ParseTimeRange() (*time.Time, *time.Time) {
	return parseTimeStr(p.StartTime), parseTimeStr(p.EndTime)
}

func parseTimeStr(v *string) *time.Time {
	if v == nil || *v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		t, err = time.Parse("2006-01-02", *v)
	}
	if err != nil {
		return nil
	}
	return &t
}

// readListParams reads ListParams from body, falls back to URL query params.
func readListParams(ctx iris.Context) (ListParams, error) {
	var p ListParams
	// Empty body is valid for list endpoints — fall back to URL params / defaults.
	if err := ctx.ReadJSON(&p); err != nil && !errors.Is(err, io.EOF) {
		slog.Error("read list params failed", "error", err)
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "invalid request body")
		return p, err
	}
	if p.Page == 0 && p.PageSize == 0 {
		// fallback to URL params
		page, _ := strconv.Atoi(ctx.URLParamDefault("page", "1"))
		pageSize, _ := strconv.Atoi(ctx.URLParamDefault("page_size", strconv.Itoa(pkg.PageDefaultSize)))
		p.Page = page
		p.PageSize = pageSize
	}
	p.Normalize()
	return p, nil
}

// readTimeRange reads startTime/endTime from body via ListParams, falls back to URL params.
func readTimeRange(ctx iris.Context, p ListParams) (*time.Time, *time.Time) {
	st, et := p.ParseTimeRange()
	if st == nil && et == nil {
		// fallback to URL params
		st = parseTimeParam(ctx, "start_time")
		et = parseTimeParam(ctx, "end_time")
	}
	return st, et
}

func parseTimeParam(ctx iris.Context, key string) *time.Time {
	v := ctx.URLParam(key)
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		t, err = time.Parse("2006-01-02", v)
	}
	if err != nil {
		return nil
	}
	return &t
}
