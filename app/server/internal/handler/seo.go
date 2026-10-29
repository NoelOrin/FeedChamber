package handler

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

/*
 * 爬虫面（sitemap / robots / RSS）的唯一归属是后端，前端不生成任何爬虫文件。
 * 这条边界由 openspec change 2026-10-28-seo-public-indexing 的 seo capability 钉住：
 * 部署层把这 6 条路径转给后端，前端路由若也实现同名路径会与之冲突。
 *
 * 为什么不做在前端：枚举全站帖子只能走 POST /posts/list，而 pkg.PageMaxSize 是 100，
 * 几千帖就是几十次后端调用；后端直查 DB 只取 id/slug/updated_at 一条 SQL 就够。
 * 详见该 change 的 design.md D1。
 */

type SEOHandler interface {
	Sitemap(ctx iris.Context)
	SitemapIndex(ctx iris.Context)
	SitemapSpaces(ctx iris.Context)
	SitemapTags(ctx iris.Context)
	Robots(ctx iris.Context)
	LatestRSS(ctx iris.Context)
}

type seoHandler struct {
	posts   service.PostService
	spaces  service.SpaceService
	tags    service.TagService
	setting service.SettingService
}

func newSEOHandler(
	posts service.PostService,
	spaces service.SpaceService,
	tags service.TagService,
	setting service.SettingService,
) SEOHandler {
	return &seoHandler{posts: posts, spaces: spaces, tags: tags, setting: setting}
}

// sitemapPageSize reads the configured shard size, clamped to the protocol limit.
// A zero/absent value must never produce an unbounded query.
func (h *seoHandler) sitemapPageSize(ctx context.Context) int {
	size := pkg.GetSettingInt(ctx, h.setting, model.SettingSitemapPageSize, "SITEMAP_PAGE_SIZE", 5000)
	if size < 1 {
		size = 5000
	}
	if size > model.SitemapMaxPageSize {
		size = model.SitemapMaxPageSize
	}
	return size
}

// postPath is the single source of truth for a post's canonical path. Sitemap,
// RSS and the index all call it so the URL shape cannot drift between them again.
func postPath(slug, id string) string {
	if slug == "" {
		slug = "post"
	}
	return "/t/" + url.PathEscape(slug) + "/" + url.PathEscape(id)
}

func spacePath(slug string) string { return "/spaces/" + url.PathEscape(slug) }

func tagPath(name string) string { return "/timeline?tag=" + url.QueryEscape(name) }

// SitemapIndex GET /sitemap.xml — exception to RPC POST rule for crawlers.
//
// @Summary      获取 sitemap 索引
// @Description  列出全部子 sitemap：帖子分片（按页）、版面、标签。爬虫入口。
// @Tags         SEO
// @Produce      xml
// @Success      200  {string}  string  "sitemap index XML"
// @Router       /sitemap.xml [get]
func (h *seoHandler) SitemapIndex(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)

	// Post shards: declare exactly as many as there are pages of content.
	total, err := h.posts.CountSitemapEntries(reqCtx)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pageSize := h.sitemapPageSize(reqCtx)
	shards := (total + pageSize - 1) / pageSize
	if shards < 1 {
		shards = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for page := 1; page <= shards; page++ {
		loc := fmt.Sprintf("%s/sitemap-posts.xml?page=%d", siteURL, page)
		b.WriteString("<sitemap><loc>" + xmlEscape(loc) + "</loc><lastmod>" + now + "</lastmod></sitemap>")
	}

	b.WriteString("<sitemap><loc>" + xmlEscape(siteURL+"/sitemap-spaces.xml") + "</loc><lastmod>" + now + "</lastmod></sitemap>")
	b.WriteString("<sitemap><loc>" + xmlEscape(siteURL+"/sitemap-tags.xml") + "</loc><lastmod>" + now + "</lastmod></sitemap>")
	b.WriteString("</sitemapindex>")

	ctx.ContentType("application/xml")
	_, _ = ctx.WriteString(b.String())
}

// Sitemap GET /sitemap-posts.xml?page=N — one shard of post URLs.
//
// @Summary      获取帖子 sitemap 分片
// @Description  按 page 分页输出帖子 URL。超出范围返回合法空 urlset（不是 404），
// @Description  避免爬虫把整个 sitemap 判为失效。
// @Tags         SEO
// @Produce      xml
// @Param        page  query  int  false  "分片页码，从 1 开始"
// @Success      200  {string}  string  "urlset XML"
// @Router       /sitemap-posts.xml [get]
func (h *seoHandler) Sitemap(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	page := 1
	if raw := ctx.URLParam("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}

	entries, err := h.posts.SitemapEntries(reqCtx, page, h.sitemapPageSize(reqCtx))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, it := range entries {
		b.WriteString("<url>")
		b.WriteString("<loc>" + xmlEscape(siteURL+postPath(it.Slug, it.ID)) + "</loc>")
		b.WriteString("<lastmod>" + it.UpdatedAt.UTC().Format(time.RFC3339) + "</lastmod>")
		b.WriteString("</url>")
	}
	b.WriteString("</urlset>")

	ctx.ContentType("application/xml")
	_, _ = ctx.WriteString(b.String())
}

// SitemapSpaces GET /sitemap-spaces.xml — every active space landing page.
//
// @Summary      获取版面 sitemap
// @Description  输出全部启用版面的首页 URL；停用版面（is_active=false）不收录。
// @Tags         SEO
// @Produce      xml
// @Success      200  {string}  string  "urlset XML"
// @Router       /sitemap-spaces.xml [get]
func (h *seoHandler) SitemapSpaces(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	var spaces []model.Space
	if h.spaces != nil {
		all, err := h.spaces.Flat(reqCtx)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		// 爬虫面固定匿名（TL0）：login / level 版面的落地页对爬虫是 403，
		// 收录进来只会制造 sitemap 与可见性不一致的坏 URL。
		idx, err := h.spaces.VisibilityIndex(reqCtx)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		for _, sp := range all {
			if !sp.IsActive || sp.Slug == "" {
				continue
			}
			if !idx.Resolve(sp.ID).Allows(0, false) {
				continue
			}
			spaces = append(spaces, sp)
		}
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, sp := range spaces {
		b.WriteString("<url>")
		b.WriteString("<loc>" + xmlEscape(siteURL+spacePath(sp.Slug)) + "</loc>")
		if !sp.UpdatedAt.IsZero() {
			b.WriteString("<lastmod>" + sp.UpdatedAt.UTC().Format(time.RFC3339) + "</lastmod>")
		}
		b.WriteString("</url>")
	}
	b.WriteString("</urlset>")

	ctx.ContentType("application/xml")
	_, _ = ctx.WriteString(b.String())
}

// SitemapTags GET /sitemap-tags.xml — every tag landing page.
//
// Tag has no slug column (model/tag.go), so the tag landing page is the filtered
// timeline rather than a /tags/{slug} route. Keep the encoding identical to what
// the frontend's nuqs reader expects.
//
// @Summary      获取标签 sitemap
// @Description  输出标签页 URL（/timeline?tag=<name>）。Tag 模型无 slug，故用查询参数。
// @Tags         SEO
// @Produce      xml
// @Success      200  {string}  string  "urlset XML"
// @Router       /sitemap-tags.xml [get]
func (h *seoHandler) SitemapTags(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	if h.tags != nil {
		// 500 covers the realistic long tail; tags beyond that are not worth a
		// dedicated crawl budget entry and would blow up the response.
		items, _, err := h.tags.List(reqCtx, 1, 500)
		if err != nil {
			pkg.HandleError(ctx, err)
			return
		}
		for _, tg := range items {
			if tg.Name == "" {
				continue
			}
			b.WriteString("<url>")
			b.WriteString("<loc>" + xmlEscape(siteURL+tagPath(tg.Name)) + "</loc>")
			b.WriteString("</url>")
		}
	}
	b.WriteString("</urlset>")

	ctx.ContentType("application/xml")
	_, _ = ctx.WriteString(b.String())
}

// Robots GET /robots.txt — exception to RPC POST rule for crawlers.
//
// @Summary      获取 robots.txt
// @Description  允许抓取公开内容，屏蔽私有面与 API；声明 sitemap 索引位置。
// @Tags         SEO
// @Produce      plain
// @Success      200  {string}  string  "robots.txt"
// @Router       /robots.txt [get]
func (h *seoHandler) Robots(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteString("Disallow: /api/\n")
	b.WriteString("Disallow: /admin/\n")
	b.WriteString("Disallow: /login\n")
	b.WriteString("Disallow: /register\n")
	b.WriteString("Disallow: /messages\n")
	b.WriteString("Disallow: /timeline/create\n")
	b.WriteString("Disallow: /home\n")
	// /user/ carries both the private account area and other people's profiles.
	// The whole prefix is blocked by default; when the operator opts into
	// indexing other people's profiles we must NOT emit the prefix-wide rule,
	// otherwise the setting would be silently inert.
	indexable := pkg.GetSettingOrDefault(reqCtx, h.setting, model.SettingUserProfileIndexable, "USER_PROFILE_INDEXABLE", "false")
	if indexable != "true" {
		b.WriteString("Disallow: /user/\n")
	}
	b.WriteString("\nSitemap: " + siteURL + "/sitemap.xml\n")

	ctx.ContentType("text/plain")
	_, _ = ctx.WriteString(b.String())
}

// LatestRSS GET /latest.rss — exception to RPC POST rule for crawlers.
//
// @Summary      获取最新帖子 RSS
// @Description  输出最近 20 条公开帖子的 RSS 2.0 feed。
// @Tags         SEO
// @Produce      xml
// @Success      200  {string}  string  "RSS XML"
// @Router       /latest.rss [get]
func (h *seoHandler) LatestRSS(ctx iris.Context) {
	reqCtx := ctx.Request().Context()
	siteURL := h.siteURL(reqCtx)

	entries, err := h.posts.SitemapEntries(reqCtx, 1, 20)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	siteName := "OrinForum"
	siteDesc := ""
	if h.setting != nil {
		if v := pkg.GetSettingOrDefault(reqCtx, h.setting, "site_name", "SITE_NAME", siteName); v != "" {
			siteName = v
		}
		siteDesc = pkg.GetSettingOrDefault(reqCtx, h.setting, model.SettingSEODescription, "", "")
		if siteDesc == "" {
			siteDesc = pkg.GetSettingOrDefault(reqCtx, h.setting, "site_description", "SITE_DESCRIPTION", "")
		}
	}

	// RSS <description> carries the post digest; the sitemap projection has no
	// digest, so fetch titles/digests for this small fixed window in one call.
	details, _ := h.posts.ListByIDs(reqCtx, entryIDs(entries))
	detailByID := make(map[string]model.PostListItem, len(details))
	for _, d := range details {
		detailByID[d.ID] = d
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>`)
	b.WriteString("<title>" + xmlEscape(siteName) + "</title>")
	b.WriteString("<link>" + xmlEscape(siteURL) + "</link>")
	b.WriteString("<description>" + xmlEscape(siteDesc) + "</description>")
	b.WriteString("<language>zh-CN</language>")
	b.WriteString("<lastBuildDate>" + time.Now().UTC().Format(time.RFC1123Z) + "</lastBuildDate>")
	b.WriteString(`<atom:link href="` + xmlEscape(siteURL+"/latest.rss") + `" rel="self" type="application/rss+xml" />`)
	for _, it := range entries {
		loc := siteURL + postPath(it.Slug, it.ID)
		d := detailByID[it.ID]
		title := d.Title
		if title == "" {
			title = it.Slug
		}
		b.WriteString("<item>")
		b.WriteString("<title>" + xmlEscape(title) + "</title>")
		b.WriteString("<link>" + xmlEscape(loc) + "</link>")
		b.WriteString("<guid>" + xmlEscape(loc) + "</guid>")
		b.WriteString("<description>" + xmlEscape(d.ContentDigest) + "</description>")
		b.WriteString("<pubDate>" + d.CreatedAt.UTC().Format(time.RFC1123Z) + "</pubDate>")
		b.WriteString("</item>")
	}
	b.WriteString("</channel></rss>")

	ctx.ContentType("application/rss+xml")
	_, _ = ctx.WriteString(b.String())
}

func entryIDs(entries []repository.SitemapEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func (h *seoHandler) siteURL(ctx context.Context) string {
	siteURL := "http://localhost:8080"
	if h.setting != nil {
		if v := pkg.GetSettingOrDefault(ctx, h.setting, "site_url", "SITE_URL", ""); v != "" {
			siteURL = strings.TrimRight(v, "/")
		}
	}
	return siteURL
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>`

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
