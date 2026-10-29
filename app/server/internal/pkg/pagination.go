package pkg

// NormalizePagination 规范化分页参数。
// 使用 PageMaxSize 和 PageDefaultSize（启动时从 settings 加载）。
func NormalizePagination(page, pageSize int) (int, int) {
	return NormalizePaginationCustom(page, pageSize, PageMaxSize, PageDefaultSize)
}

// NormalizePaginationCustom 支持自定义上限和默认值的分页规范化。
func NormalizePaginationCustom(page, pageSize, maxSize, defaultSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxSize {
		pageSize = defaultSize
	}
	return page, pageSize
}
