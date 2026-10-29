package model

import (
	"time"

	"orinforum/internal/pkg"
)

// NewUser creates a User with generated ID, timestamps and default role.
func NewUser(username, email string) *User {
	now := time.Now()
	return &User{
		ID:          pkg.NewID(),
		Username:    username,
		DisplayName: username,
		Email:       email,
		Role:        "user",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewPost creates a Post with generated ID, content digest, timestamps and default status.
func NewPost(authorID, title, body, spaceID string) (*Post, *Content) {
	now := time.Now()
	contentID := pkg.NewID()

	content := &Content{
		ID:        contentID,
		AuthorID:  authorID,
		Body:      body,
		RootID:    contentID,
		ParentID:  "",
		Status:    PostStatusApproved,
		CreatedAt: now,
		UpdatedAt: now,
	}

	post := &Post{
		ID:            pkg.NewID(),
		Title:         title,
		ContentID:     contentID,
		ContentDigest: DigestBody(body, 200),
		AuthorID:      authorID,
		SpaceID:       spaceID,
		Status:        PostStatusApproved,
		LastActiveAt:  now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return post, content
}

// NewContent creates a Content with generated ID and timestamps.
func NewContent(authorID, body, rootID, parentID string) *Content {
	now := time.Now()
	return &Content{
		ID:        pkg.NewID(),
		AuthorID:  authorID,
		Body:      body,
		RootID:    rootID,
		ParentID:  parentID,
		Status:    PostStatusApproved,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewBanned creates a Banned record with generated ID and timestamps.
func NewBanned(userID, scope, spaceID, reason, createdBy string, isPermanent bool, expiresAt *time.Time) *Banned {
	now := time.Now()
	return &Banned{
		ID:          pkg.NewID(),
		UserID:      userID,
		Scope:       scope,
		SpaceID:     spaceID,
		Reason:      reason,
		IsPermanent: isPermanent,
		ExpiresAt:   expiresAt,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewNotification creates a Notification with generated ID and timestamp.
func NewNotification(userID, notifType, content, referenceID, senderID string) *Notification {
	return &Notification{
		ID:          pkg.NewID(),
		UserID:      userID,
		Type:        notifType,
		Content:     content,
		ReferenceID: referenceID,
		SenderID:    senderID,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}
}

// NewRole creates a Role with generated ID and timestamp.
func NewRole(name string) *Role {
	return &Role{
		ID:        pkg.NewID(),
		Name:      name,
		CreatedAt: time.Now(),
	}
}

// DigestBody truncates body text to maxLen runes for use as content digest.
func DigestBody(body string, maxLen int) string {
	runes := []rune(body)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return body
}

// NewGroup creates a Group with generated ID and timestamp.
func NewGroup(name, displayName, visibility string) *Group {
	if visibility == "" {
		visibility = GroupVisibilityPublic
	}
	return &Group{
		ID:          pkg.NewID(),
		Name:        name,
		DisplayName: displayName,
		Visibility:  visibility,
		CreatedAt:   time.Now(),
	}
}

// NewGroupMember creates a GroupMember with default role and timestamp.
func NewGroupMember(groupID, userID, role string) *GroupMember {
	if role == "" {
		role = GroupMemberRoleMember
	}
	return &GroupMember{
		GroupID:  groupID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}
}
