package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	importDir    string
	importSource string
)

type discourseUser struct {
	ID       any    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Bio      string `json:"bio"`
	Avatar   string `json:"avatar"`
}

type discourseCategory struct {
	ID          any    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

type discourseTopic struct {
	ID         any    `json:"id"`
	Title      string `json:"title"`
	CategoryID any    `json:"category_id"`
	UserID     any    `json:"user_id"`
	CreatedAt  string `json:"created_at"`
}

type discoursePost struct {
	ID         any    `json:"id"`
	TopicID    any    `json:"topic_id"`
	UserID     any    `json:"user_id"`
	PostNumber int    `json:"post_number"`
	Raw        string `json:"raw"`
	Cooked     string `json:"cooked"`
	ReplyTo    any    `json:"reply_to_post_number"`
	CreatedAt  string `json:"created_at"`
}

var importDiscourseCmd = &cobra.Command{
	Use:   "import-discourse",
	Short: "Import Discourse JSON export directory (users/categories/topics/posts)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(importDir) == "" {
			return fmt.Errorf("--dir is required")
		}
		if importSource == "" {
			importSource = "discourse"
		}
		cfg := config.Load()
		store, err := db.New(cfg.DB)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		if store.SQL() == nil {
			return fmt.Errorf("import-discourse MVP currently supports SQL backends only")
		}
		gdb := store.SQL().DB()
		ctx := context.Background()
		if err := store.AutoMigrate(
			&model.User{},
			&model.Credential{},
			&model.Space{},
			&model.Post{},
			&model.Content{},
			&model.ImportMap{},
			&model.Meta{},
		); err != nil {
			return err
		}

		users, err := readJSONArray[discourseUser](filepath.Join(importDir, "users.json"))
		if err != nil {
			return err
		}
		cats, err := readJSONArray[discourseCategory](filepath.Join(importDir, "categories.json"))
		if err != nil {
			return err
		}
		topics, err := readJSONArray[discourseTopic](filepath.Join(importDir, "topics.json"))
		if err != nil {
			return err
		}
		posts, err := readJSONArray[discoursePost](filepath.Join(importDir, "posts.json"))
		if err != nil {
			return err
		}

		userMap := map[string]string{}
		spaceMap := map[string]string{}
		topicMap := map[string]string{}         // topic ext -> post local
		topicRoot := map[string]string{}        // topic ext -> root content local
		postNumToContent := map[string]string{} // topic:post_number -> content id

		// users
		for _, u := range users {
			ext := anyString(u.ID)
			if ext == "" {
				continue
			}
			if local, ok, err := lookupImport(ctx, gdb, importSource, "user", ext); err != nil {
				return err
			} else if ok {
				userMap[ext] = local
				continue
			}
			username := strings.TrimSpace(u.Username)
			if username == "" {
				username = "user_" + ext
			}
			email := strings.TrimSpace(u.Email)
			if email == "" {
				email = fmt.Sprintf("imported+%s@example.invalid", ext)
			}
			// ensure unique username/email if collision
			username = uniqueUsername(ctx, gdb, username)
			email = uniqueEmail(ctx, gdb, email)
			nu := model.NewUser(username, email)
			if strings.TrimSpace(u.Name) != "" {
				nu.DisplayName = u.Name
			}
			if u.Bio != "" {
				nu.Bio = u.Bio
			}
			if u.Avatar != "" {
				nu.Avatar = u.Avatar
			}
			if err := gdb.WithContext(ctx).Create(nu).Error; err != nil {
				return fmt.Errorf("create user %s: %w", username, err)
			}
			// random password + force first change via meta best-effort
			pw := randomPassword()
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			cred := &model.Credential{UserID: nu.ID, PasswordHash: string(hash), UpdatedAt: time.Now()}
			if err := gdb.WithContext(ctx).Create(cred).Error; err != nil {
				return err
			}
			_ = gdb.WithContext(ctx).Create(&model.Meta{Key: model.MustChangePasswordKey(nu.ID), Value: "true"}).Error
			if err := saveImport(ctx, gdb, importSource, "user", ext, nu.ID); err != nil {
				return err
			}
			userMap[ext] = nu.ID
		}

		// categories -> spaces
		for _, c := range cats {
			ext := anyString(c.ID)
			if ext == "" {
				continue
			}
			if local, ok, err := lookupImport(ctx, gdb, importSource, "space", ext); err != nil {
				return err
			} else if ok {
				spaceMap[ext] = local
				continue
			}
			name := strings.TrimSpace(c.Name)
			if name == "" {
				name = "category-" + ext
			}
			slug := strings.TrimSpace(c.Slug)
			if slug == "" {
				slug = "cat-" + ext
			}
			sp := &model.Space{
				ID:          pkg.NewID(),
				Name:        name,
				Slug:        slug,
				Description: c.Description,
				SortOrder:   c.Position,
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := gdb.WithContext(ctx).Create(sp).Error; err != nil {
				// slug collision: suffix
				sp.Slug = slug + "-" + ext
				sp.Name = name + "-" + ext
				if err := gdb.WithContext(ctx).Create(sp).Error; err != nil {
					return fmt.Errorf("create space %s: %w", name, err)
				}
			}
			if err := saveImport(ctx, gdb, importSource, "space", ext, sp.ID); err != nil {
				return err
			}
			spaceMap[ext] = sp.ID
		}

		// index posts by topic
		postsByTopic := map[string][]discoursePost{}
		for _, p := range posts {
			tid := anyString(p.TopicID)
			postsByTopic[tid] = append(postsByTopic[tid], p)
		}
		for tid := range postsByTopic {
			sort.Slice(postsByTopic[tid], func(i, j int) bool {
				return postsByTopic[tid][i].PostNumber < postsByTopic[tid][j].PostNumber
			})
		}

		// topics + posts
		for _, t := range topics {
			ext := anyString(t.ID)
			if ext == "" {
				continue
			}
			if local, ok, err := lookupImport(ctx, gdb, importSource, "post", ext); err != nil {
				return err
			} else if ok {
				topicMap[ext] = local
				// still load root mapping if present
				if root, ok2, _ := lookupImport(ctx, gdb, importSource, "topic_root", ext); ok2 {
					topicRoot[ext] = root
				}
				continue
			}
			authorExt := anyString(t.UserID)
			authorID := userMap[authorExt]
			if authorID == "" {
				// fallback first user or skip
				continue
			}
			spaceID := spaceMap[anyString(t.CategoryID)]
			title := strings.TrimSpace(t.Title)
			if title == "" {
				title = "topic-" + ext
			}
			body := ""
			tPosts := postsByTopic[ext]
			if len(tPosts) > 0 {
				body = firstNonEmpty(tPosts[0].Raw, tPosts[0].Cooked)
			}
			if body == "" {
				body = title
			}
			post, content := model.NewPost(authorID, title, body, spaceID)
			if ts := parseTime(t.CreatedAt); !ts.IsZero() {
				post.CreatedAt = ts
				post.UpdatedAt = ts
				post.LastActiveAt = ts
				content.CreatedAt = ts
				content.UpdatedAt = ts
			}
			if err := gdb.WithContext(ctx).Create(content).Error; err != nil {
				return fmt.Errorf("create topic content: %w", err)
			}
			if err := gdb.WithContext(ctx).Create(post).Error; err != nil {
				return fmt.Errorf("create topic post: %w", err)
			}
			if err := saveImport(ctx, gdb, importSource, "post", ext, post.ID); err != nil {
				return err
			}
			if err := saveImport(ctx, gdb, importSource, "topic_root", ext, content.ID); err != nil {
				return err
			}
			topicMap[ext] = post.ID
			topicRoot[ext] = content.ID
			if len(tPosts) > 0 {
				postNumToContent[ext+":"+strconv.Itoa(tPosts[0].PostNumber)] = content.ID
				_ = saveImport(ctx, gdb, importSource, "content", anyString(tPosts[0].ID), content.ID)
			}

			// replies (post_number > 1)
			for _, p := range tPosts {
				if p.PostNumber <= 1 {
					continue
				}
				pext := anyString(p.ID)
				if pext != "" {
					if _, ok, err := lookupImport(ctx, gdb, importSource, "content", pext); err != nil {
						return err
					} else if ok {
						continue
					}
				}
				aext := anyString(p.UserID)
				aid := userMap[aext]
				if aid == "" {
					aid = authorID
				}
				rootID := topicRoot[ext]
				parentID := rootID
				if rn := anyString(p.ReplyTo); rn != "" {
					if pid, ok := postNumToContent[ext+":"+rn]; ok {
						parentID = pid
					}
				}
				body := firstNonEmpty(p.Raw, p.Cooked)
				if body == "" {
					body = "(empty)"
				}
				c := model.NewContent(aid, body, rootID, parentID)
				if ts := parseTime(p.CreatedAt); !ts.IsZero() {
					c.CreatedAt = ts
					c.UpdatedAt = ts
				}
				if err := gdb.WithContext(ctx).Create(c).Error; err != nil {
					return fmt.Errorf("create reply content: %w", err)
				}
				postNumToContent[ext+":"+strconv.Itoa(p.PostNumber)] = c.ID
				if pext != "" {
					if err := saveImport(ctx, gdb, importSource, "content", pext, c.ID); err != nil {
						return err
					}
				}
				// bump comment count best-effort
				_ = gdb.WithContext(ctx).Model(&model.Post{}).Where("id = ?", post.ID).
					UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "import complete: users=%d spaces=%d topics=%d posts=%d\n",
			len(userMap), len(spaceMap), len(topicMap), len(posts))
		return nil
	},
}

func readJSONArray[T any](path string) ([]T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var arr []T
	if err := json.Unmarshal(b, &arr); err != nil {
		// sometimes export is { "users": [...] }
		var wrap map[string]json.RawMessage
		if err2 := json.Unmarshal(b, &wrap); err2 == nil {
			for _, v := range wrap {
				if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
					return arr, nil
				}
			}
		}
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return arr, nil
}

func anyString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func lookupImport(ctx context.Context, gdb *gorm.DB, source, entity, ext string) (string, bool, error) {
	var m model.ImportMap
	err := gdb.WithContext(ctx).Where("source = ? AND entity_type = ? AND external_id = ?", source, entity, ext).Limit(1).Find(&m).Error
	if err == nil && m.LocalID == "" {
		return "", false, nil
	}
	if err == gorm.ErrRecordNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.LocalID, true, nil
}

func saveImport(ctx context.Context, gdb *gorm.DB, source, entity, ext, local string) error {
	m := model.ImportMap{
		ID:         pkg.NewID(),
		Source:     source,
		EntityType: entity,
		ExternalID: ext,
		LocalID:    local,
		CreatedAt:  time.Now(),
	}
	return gdb.WithContext(ctx).Create(&m).Error
}

func uniqueUsername(ctx context.Context, gdb *gorm.DB, base string) string {
	name := base
	for i := 0; i < 50; i++ {
		var count int64
		_ = gdb.WithContext(ctx).Model(&model.User{}).Where("username = ?", name).Count(&count).Error
		if count == 0 {
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i+1)
	}
	return base + "_" + pkg.NewID()[:6]
}

func uniqueEmail(ctx context.Context, gdb *gorm.DB, base string) string {
	email := base
	for i := 0; i < 50; i++ {
		var count int64
		_ = gdb.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error
		if count == 0 {
			return email
		}
		at := strings.Index(base, "@")
		if at <= 0 {
			email = fmt.Sprintf("%s+%d@example.invalid", base, i+1)
		} else {
			email = fmt.Sprintf("%s+%d%s", base[:at], i+1, base[at:])
		}
	}
	return "imported+" + pkg.NewID()[:8] + "@example.invalid"
}

func randomPassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05.000Z"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func init() {
	importDiscourseCmd.Flags().StringVar(&importDir, "dir", "", "Discourse JSON export directory")
	importDiscourseCmd.Flags().StringVar(&importSource, "source", "discourse", "import source key for import_maps")
	RootCmd.AddCommand(importDiscourseCmd)
}
