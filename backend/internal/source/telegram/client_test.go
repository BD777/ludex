package telegramsource

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestRelatedMediaMessagesFromPoolDoesNotBorrowCoverForDocumentMessage(t *testing.T) {
	selected := &tg.Message{
		ID:    10,
		Media: &tg.MessageMediaDocument{Document: &tg.DocumentEmpty{ID: 1}},
	}

	related := relatedMediaMessagesFromPool(selected, []*tg.Message{
		{ID: 9, Media: &tg.MessageMediaPhoto{}},
		selected,
	})

	if len(related) != 1 {
		t.Fatalf("related message count = %d", len(related))
	}
	if related[0] != selected {
		t.Fatal("document message should not borrow nearby media")
	}
}

func TestRelatedMediaMessagesFromPoolDoesNotBorrowCoverForSinglePhotoMessage(t *testing.T) {
	selected := &tg.Message{
		ID:    10,
		Media: &tg.MessageMediaPhoto{},
	}

	related := relatedMediaMessagesFromPool(selected, []*tg.Message{
		{ID: 9, Media: &tg.MessageMediaPhoto{}},
		selected,
		{ID: 11, Media: &tg.MessageMediaPhoto{}},
	})

	if len(related) != 1 {
		t.Fatalf("related message count = %d", len(related))
	}
	if related[0] != selected {
		t.Fatal("single-photo message should use its own media only")
	}
}
