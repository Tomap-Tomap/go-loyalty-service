package tokenworker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenWorker_GetSubFromToken(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		tw := NewToken("test", 3*time.Hour)
		token, err := tw.GetToken("test")
		require.NoError(t, err)
		_, b := tw.GetSubFromToken(token + "1")
		require.False(t, b)
	})

	t.Run("positive test", func(t *testing.T) {
		tw := NewToken("test", 3*time.Hour)
		token, err := tw.GetToken("test")
		require.NoError(t, err)
		s, b := tw.GetSubFromToken(token)
		require.True(t, b)
		require.Equal(t, "test", s)
	})
}
