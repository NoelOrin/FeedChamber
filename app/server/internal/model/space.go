package model

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

const (
	maxHierarchyWalk    = 32
	maxAncestorChainLen = 1000
)

// 版面可见性策略（取代 auth_required 布尔开关）。
//
// public：匿名可读；login：需登录；level：需等级 ≥ MinLevel。
// 子版面继承祖先策略：取整条祖先链上最严格的一档（level > login > public），
// MinLevel 取链上最大值。见 service/visibility.go。
const (
	SpaceVisibilityPublic = "public"
	SpaceVisibilityLogin  = "login"
	SpaceVisibilityLevel  = "level"
)

type Space struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	Icon        string `gorm:"size:32" json:"icon"`
	Slug        string `gorm:"uniqueIndex;not null" json:"slug"`
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
	ParentID    string `gorm:"index" json:"parent_id"`
	Depth       int    `gorm:"default:0;index" json:"depth"`
	Path        string `gorm:"size:1024;index" json:"path"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	// AuthRequired 是 2026-10-28 之前的登录开关，已由 Visibility 取代；
	// 启动迁移会把 true 转成 login 并清零本列，仅为兼容旧库保留。
	AuthRequired bool `gorm:"default:false" json:"auth_required"`
	// Visibility ∈ public | login | level；MinLevel 仅 level 档生效。
	Visibility string `gorm:"size:16;default:public;index" json:"visibility"`
	MinLevel   int    `gorm:"default:0" json:"min_level"`
	System     bool   `gorm:"default:false;index" json:"system"` // 系统版面（如暗区），不可被普通用户管理
	// Recommendable 是否进入主推荐流（暗区=false）。
	//
	// DB 默认必须是 false：GORM 在 Create 时省略零值，若默认写成 true，
	// 显式传 false 的暗区会被数据库默认值改回 true（实测踩过）。
	// 普通版面在 seed/创建处显式写 true（非零值必然入库），存量行由
	// backfillSpaceRecommendable 一次性补齐。
	Recommendable bool           `gorm:"default:false;index" json:"recommendable"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

type SpaceNode struct {
	Space
	Children        []*SpaceNode `json:"children"`
	Level           int          `json:"level"`
	HasChildren     bool         `json:"has_children"`
	ChildrenCount   int          `json:"children_count"`
	DescendantCount int          `json:"descendant_count"`
}

type SpaceBreadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Icon string `json:"icon"`
}

func BuildSpaceTree(spaces []Space) []*SpaceNode {
	if len(spaces) == 0 {
		return []*SpaceNode{}
	}
	nodeByID := make(map[string]*SpaceNode, len(spaces))
	for _, s := range spaces {
		sp := s
		n := &SpaceNode{Space: sp, Level: sp.Depth, HasChildren: false}
		nodeByID[sp.ID] = n
	}
	var roots []*SpaceNode
	for _, n := range nodeByID {
		if n.ParentID == "" {
			roots = append(roots, n)
			continue
		}
		parent, ok := nodeByID[n.ParentID]
		if !ok || parent.ID == n.ID {
			roots = append(roots, n)
			continue
		}
		cur := parent
		isCycle := false
		seen := map[string]bool{n.ID: true}
		for cur != nil {
			if seen[cur.ID] {
				isCycle = true
				break
			}
			seen[cur.ID] = true
			if cur.ParentID == "" {
				break
			}
			cur = nodeByID[cur.ParentID]
		}
		if isCycle {
			roots = append(roots, n)
			continue
		}
		parent.Children = append(parent.Children, n)
		parent.HasChildren = true
	}
	var sortNodes func(nodes []*SpaceNode)
	sortNodes = func(nodes []*SpaceNode) {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].SortOrder != nodes[j].SortOrder {
				return nodes[i].SortOrder < nodes[j].SortOrder
			}
			return nodes[i].Name < nodes[j].Name
		})
		for _, n := range nodes {
			if len(n.Children) > 0 {
				sortNodes(n.Children)
			}
		}
	}
	sortNodes(roots)
	var dfs func(nodes []*SpaceNode, level int) int
	dfs = func(nodes []*SpaceNode, level int) int {
		total := 0
		for _, n := range nodes {
			n.Level = level
			n.HasChildren = len(n.Children) > 0
			n.ChildrenCount = len(n.Children)
			if n.HasChildren {
				sub := dfs(n.Children, level+1)
				n.DescendantCount = sub
				total += 1 + sub
			} else {
				n.DescendantCount = 0
				total += 1
			}
		}
		return total
	}
	dfs(roots, 0)
	return roots
}

func CollectDescendantIDs(spaces []Space, spaceID string) []string {
	if spaceID == "" || len(spaces) == 0 {
		return nil
	}
	childrenOf := make(map[string][]string, len(spaces))
	for _, s := range spaces {
		if s.ParentID != "" {
			childrenOf[s.ParentID] = append(childrenOf[s.ParentID], s.ID)
		}
	}
	var result []string
	queue := []string{spaceID}
	visited := map[string]bool{spaceID: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, childID := range childrenOf[cur] {
			if visited[childID] {
				continue
			}
			visited[childID] = true
			result = append(result, childID)
			queue = append(queue, childID)
		}
	}
	return result
}

func CollectAncestorIDs(spaces []Space, spaceID string) []string {
	if spaceID == "" {
		return nil
	}
	byID := make(map[string]Space, len(spaces))
	for _, s := range spaces {
		byID[s.ID] = s
	}
	var chain []string
	curID := spaceID
	seen := map[string]bool{}
	for {
		sp, ok := byID[curID]
		if !ok {
			break
		}
		pid := sp.ParentID
		if pid == "" {
			break
		}
		if seen[pid] {
			break
		}
		seen[pid] = true
		chain = append(chain, pid)
		curID = pid
		if len(chain) > maxAncestorChainLen {
			break
		}
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func BuildPath(ancestorIDs []string, selfID string) string {
	if selfID == "" {
		return "/"
	}
	if len(ancestorIDs) == 0 {
		return "/" + selfID
	}
	path := ""
	for _, id := range ancestorIDs {
		path += "/" + id
	}
	path += "/" + selfID
	return path
}
