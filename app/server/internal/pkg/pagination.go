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

// Offset 由 page/pageSize 计算 SQL offset。
//
// 单独成函数是因为 `(page-1)*pageSize` 在 page 取极大值时整型溢出为负数，
// PostgreSQL 会直接报 "OFFSET must not be negative"（500 而不是空列表）。
// 用 int64 计算并把上界夹在 int32 安全区，任何输入都只会得到合法 offset。
func Offset(page, pageSize int) int {
	const maxOffset = 1 << 31
	if page < 1 || pageSize < 1 {
		return 0
	}
	// 先判页号上界再相乘：int64 乘法在 page 取极大值时同样会溢出成负数。
	if page > maxOffset/pageSize {
		return maxOffset
	}
	off := (page - 1) * pageSize
	if off > maxOffset {
		return maxOffset
	}
	return off
}
