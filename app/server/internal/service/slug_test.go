package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugifyTitle(t *testing.T) {
	assert.Equal(t, "hello-world", SlugifyTitle("Hello World"))
	assert.Equal(t, "post", SlugifyTitle("!!!"))
	assert.Equal(t, "a-b", SlugifyTitle(" a   b "))
}
