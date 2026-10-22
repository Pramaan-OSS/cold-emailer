package worker

import (
	"context"
	"errors"

	"github.com/pramaan-oss/cold-email/internal/models"
)

func (w *WorkerService) HandleMessageFolder(ctx context.Context, action models.MessageFolderAction) error {
	mail, ok := w.loadedMailbox(ctx, action.EmailID)
	if !ok {
		return errors.New("The mailbox is not loaded yet.")
	}
	for _, m := range action.Messages {
		id := m.RFCMessageID
		if id == "" {
			id = m.ProviderID
		}
		if e := mail.ChangeLabels(ctx, id, []string{action.Folder}, []string{"inbox"}, "folder-"+action.EmailID.String()+"-"+id+"-"+action.Folder); e != nil {
			return e
		}
	}
	return nil
}
