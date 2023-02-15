package pgsnap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getFilename(t *testing.T) {
	s := &script{t: t}
	assert.Equal(t, "pgsnap__getfilename.txt", s.getFilename())

	t.Run("another test name", func(t *testing.T) {
		s = &script{t: t}
		assert.Equal(t, "pgsnap__getfilename__another_test_name.txt", s.getFilename())
	})

	t.Run("what about this one?", func(t *testing.T) {
		s = &script{t: t}
		assert.Equal(t, "pgsnap__getfilename__what_about_this_one_.txt", s.getFilename())
	})
}
