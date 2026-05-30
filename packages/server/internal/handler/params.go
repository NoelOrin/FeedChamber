package handler

import (
	"strconv"
	"time"

	"github.com/kataras/iris/v12"
)

// ListParams is the common pagination + time filter request body for list/search endpoints.
type ListParams struct {
	Page      int     `json:"page"`
	PageSize  int     `json:"pageSize"`
	StartTime *string `json:"startTime"`
	EndTime   *string `json:"endTime"`
}

func (p *ListParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		p.PageSize = 20
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
func readListParams(ctx iris.Context) ListParams {
	var p ListParams
	_ = ctx.ReadJSON(&p)
	if p.Page == 0 && p.PageSize == 0 {
		// fallback to URL params
		page, _ := strconv.Atoi(ctx.URLParamDefault("page", "1"))
		pageSize, _ := strconv.Atoi(ctx.URLParamDefault("pageSize", "20"))
		p.Page = page
		p.PageSize = pageSize
	}
	p.Normalize()
	return p
}

// readTimeRange reads startTime/endTime from body via ListParams, falls back to URL params.
func readTimeRange(ctx iris.Context, p ListParams) (*time.Time, *time.Time) {
	st, et := p.ParseTimeRange()
	if st == nil && et == nil {
		// fallback to URL params
		st = parseTimeParam(ctx, "startTime")
		et = parseTimeParam(ctx, "endTime")
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
