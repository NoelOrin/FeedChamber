package service

import (
	"context"
	"regexp"
	"strings"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/repository"
)

var mentionRe = regexp.MustCompile(`(?:^|[\s(])@([A-Za-z0-9_]{2,32})`)

// ExtractMentions returns unique @usernames from body.
// Dedupes case-insensitively but preserves first-seen casing for store lookup.
func ExtractMentions(body string) []string {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	seen := map[string]struct{}{}
	var out []string
	for _, m := range matches {
		raw := m[1]
		key := strings.ToLower(raw)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, raw)
	}
	return out
}

// publishUserMentions resolves mentioned usernames and publishes EventUserMentioned
// for each valid user other than the author. reference_id is postID.
//
// notified accumulates the recipients handled for the current event. One comment
// can reach the same person as the post author, as a mentioned user and as a
// follower of the post/tag; without this the recipient got three notifications
// for a single comment. Callers pass the set that earlier fanout steps filled and
// this one skips (and then adds to) it.
func publishUserMentions(bus *eventbus.EventBus, userRepo repository.UserRepository, ctx context.Context, authorID, postID, text, senderName string, notified map[string]struct{}) {
	if bus == nil || userRepo == nil || text == "" {
		return
	}
	snippet := model.DigestBody(text, 80)
	for _, name := range ExtractMentions(text) {
		mentioned, err := userRepo.FindByUsername(ctx, name)
		if err != nil || mentioned == nil || mentioned.ID == "" || mentioned.ID == authorID {
			continue
		}
		if _, dup := notified[mentioned.ID]; dup {
			continue
		}
		notified[mentioned.ID] = struct{}{}
		bus.Publish(eventbus.Event{
			Type: eventbus.EventUserMentioned,
			Ctx:  ctx,
			Payload: map[string]any{
				"user_id":     mentioned.ID,
				"sender_id":   authorID,
				"sender_name": senderName,
				"post_id":     postID,
				"snippet":     snippet,
			},
		})
	}
}
