package service

import "orinforum/internal/model"

// SpaceVisibility 是版面可见性的**有效值**（已沿祖先链继承）。
type SpaceVisibility struct {
	Visibility string
	MinLevel   int
}

// VisibilityIndex 把一个用户的可见性判定收敛成单一谓词：
// 版面策略（含祖先继承）、帖子覆盖、游客 = TL0、作者例外都在这里解释。
//
// 六个内容出口（列表 / 详情 / 搜索 / sitemap / RSS / 批量）必须共用它，
// 不允许各出口自行拼装过滤条件。
type VisibilityIndex map[string]SpaceVisibility

func visibilityRank(v string) int {
	switch v {
	case model.SpaceVisibilityLevel:
		return 2
	case model.SpaceVisibilityLogin:
		return 1
	default:
		return 0
	}
}

// BuildVisibilityIndex 由全部版面构建有效可见性索引。
//
// 继承规则：取自身与整条祖先链上最严格的一档（level > login > public）；
// 同为 level 时取最大 MinLevel。环由 maxHierarchyWalk 兜底，数据异常不会死循环。
func BuildVisibilityIndex(spaces []model.Space) VisibilityIndex {
	byID := make(map[string]model.Space, len(spaces))
	for _, sp := range spaces {
		byID[sp.ID] = sp
	}
	idx := make(VisibilityIndex, len(spaces))
	for _, sp := range spaces {
		effective := SpaceVisibility{Visibility: normalizeVisibility(sp.Visibility), MinLevel: sp.MinLevel}
		current := sp
		for i := 0; i < 32; i++ {
			if current.ParentID == "" {
				break
			}
			parent, ok := byID[current.ParentID]
			if !ok {
				break
			}
			candidate := SpaceVisibility{Visibility: normalizeVisibility(parent.Visibility), MinLevel: parent.MinLevel}
			if visibilityRank(candidate.Visibility) > visibilityRank(effective.Visibility) {
				effective.Visibility = candidate.Visibility
				effective.MinLevel = candidate.MinLevel
			} else if candidate.Visibility == model.SpaceVisibilityLevel &&
				effective.Visibility == model.SpaceVisibilityLevel && candidate.MinLevel > effective.MinLevel {
				effective.MinLevel = candidate.MinLevel
			}
			current = parent
		}
		idx[sp.ID] = effective
	}
	return idx
}

func normalizeVisibility(v string) string {
	switch v {
	case model.SpaceVisibilityLogin, model.SpaceVisibilityLevel:
		return v
	default:
		return model.SpaceVisibilityPublic
	}
}

// Resolve 返回某版面的有效可见性；未登记的版面按 public 处理
// （帖子可以没有版面，历史数据也可能指向已删除版面）。
func (idx VisibilityIndex) Resolve(spaceID string) SpaceVisibility {
	if spaceID == "" {
		return SpaceVisibility{Visibility: model.SpaceVisibilityPublic}
	}
	if v, ok := idx[spaceID]; ok {
		return v
	}
	return SpaceVisibility{Visibility: model.SpaceVisibilityPublic}
}

// VisibleSpaceIDs 返回某访问者可以读取内容的版面 ID 列表。
func (idx VisibilityIndex) VisibleSpaceIDs(viewerLevel int, loggedIn bool) []string {
	out := make([]string, 0, len(idx))
	for id, v := range idx {
		if v.Allows(viewerLevel, loggedIn) {
			out = append(out, id)
		}
	}
	return out
}

// Allows 判定该有效策略是否放行某访问者。游客一律按等级 0 参与比较。
func (v SpaceVisibility) Allows(viewerLevel int, loggedIn bool) bool {
	switch v.Visibility {
	case model.SpaceVisibilityLogin:
		return loggedIn
	case model.SpaceVisibilityLevel:
		return viewerLevel >= v.MinLevel
	default:
		return true
	}
}

// RequiredLevel 返回查看该版面所需的最低等级（login 视为 1）；public 返回 0。
func (v SpaceVisibility) RequiredLevel() int {
	switch v.Visibility {
	case model.SpaceVisibilityLogin:
		return 1
	case model.SpaceVisibilityLevel:
		if v.MinLevel < 1 {
			return 1
		}
		return v.MinLevel
	default:
		return 0
	}
}

// PostRequiredLevel 返回某篇帖子对外的门槛：帖子覆盖与版面策略取更高者。
func (idx VisibilityIndex) PostRequiredLevel(spaceID string, postMinLevel int) int {
	need := idx.Resolve(spaceID).RequiredLevel()
	if postMinLevel > need {
		need = postMinLevel
	}
	return need
}
