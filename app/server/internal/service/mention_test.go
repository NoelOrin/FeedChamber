package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMentions(t *testing.T) {
	got := ExtractMentions("hi @Alice and @bob and @Alice")
	assert.Equal(t, []string{"Alice", "bob"}, got)
}

func TestExtractMentions_Boundaries(t *testing.T) {
	got := ExtractMentions("@start mid @ok ( @paren) email@x.com @a")
	// @a is length 1 (<2) skipped; email@x.com is not a mention boundary
	assert.Equal(t, []string{"start", "ok", "paren"}, got)
}

func TestExtractMentions_Empty(t *testing.T) {
	assert.Nil(t, ExtractMentions(""))
	assert.Nil(t, ExtractMentions("no mentions here"))
}
