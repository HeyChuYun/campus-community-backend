package logic

import (
	"testing"

	"hfjj/backend/proto/contentpb"
)

func TestNormalizeContentType(t *testing.T) {
	t.Run("text post", func(t *testing.T) {
		got := normalizeContentType(nil)
		if got != contentpb.PostMediaType(1) {
			t.Fatalf("expected text content_type=1, got=%d", got)
		}
	})

	t.Run("image post", func(t *testing.T) {
		got := normalizeContentType([]*contentpb.PostMedia{
			{MediaType: contentpb.PostMediaType_POST_MEDIA_TYPE_IMAGE},
		})
		if got != contentpb.PostMediaType(2) {
			t.Fatalf("expected image content_type=2, got=%d", got)
		}
	})

	t.Run("video post", func(t *testing.T) {
		got := normalizeContentType([]*contentpb.PostMedia{
			{MediaType: contentpb.PostMediaType_POST_MEDIA_TYPE_VIDEO},
		})
		if got != contentpb.PostMediaType(3) {
			t.Fatalf("expected video content_type=3, got=%d", got)
		}
	})

	t.Run("gif post", func(t *testing.T) {
		got := normalizeContentType([]*contentpb.PostMedia{
			{MediaType: contentpb.PostMediaType_POST_MEDIA_TYPE_GIF},
		})
		if got != contentpb.PostMediaType(4) {
			t.Fatalf("expected gif content_type=4, got=%d", got)
		}
	})
}
