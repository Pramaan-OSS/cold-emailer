package worker

import (
	"context"
	"errors"
	"strconv"

	"github.com/pramaan-oss/cold-email/internal/models"
)

func (w *WorkerService) HandleMessageSeen(ctx context.Context, action models.MessageSeenAction) error {
	mail, ok := w.loadedMailbox(ctx, action.EmailID)
	if !ok {
		return errors.New("The mailbox is not loaded yet.")
	}
	for _, m := range action.Messages {
		id := m.RFCMessageID
		if id == "" {
			id = m.ProviderID
		}
		add, remove := []string{}, []string{}
		if action.Seen {
			remove = append(remove, "unread")
		} else {
			add = append(add, "unread")
		}
		if e := mail.ChangeLabels(ctx, id, add, remove, "seen-"+action.EmailID.String()+"-"+id+"-"+strconv.FormatBool(action.Seen)); e != nil {
			return e
		}
	}
	return nil
}
