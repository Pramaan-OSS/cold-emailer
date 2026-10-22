package imap

import "strings"

var (
	// ImapSpam is also the guard on the warmup "remove from spam" action, so
	// a name that is merely spam-ish (a user folder called "Spam reports")
	// must not appear here.
	ImapSpam = []string{
		"spam", "junk", "junk e-mail", "junk email", "bulk mail",
		"correo no deseado", "no deseado", // Spanish
		"courrier indésirable", "indésirables", "pourriel", // French
		"junk-e-mail", "spamverdacht", // German, Dutch
		"lixo eletrônico", "lixo electrónico", // Portuguese
		"posta indesiderata",                        // Italian
		"skräppost", "uønsket e-post", "roskaposti", // Swedish, Norwegian, Finnish
		"spam-mappe", "levélszemét", // Danish-ish, Hungarian
		"niechciane", "spam-post", // Polish
		"nevyžádaná pošta", // Czech
	}
	ImapSent = []string{
		"sent", "sent mail", "sent items", "sent messages",
		"enviados", "elementos enviados", "correo enviado", // Spanish
		"éléments envoyés", "messages envoyés", "envoyés", // French
		"gesendet", "gesendete elemente", "gesendete objekte", // German
		"verzonden", "verzonden items", // Dutch
		"itens enviados", "enviadas", // Portuguese
		"posta inviata", "inviata", // Italian
		"skickat", "skickade objekt", "sendt", "lähetetyt", // Nordic
		"elküldött elemek", "elküldött üzenetek", // Hungarian
		"elementy wysłane", "wysłane", // Polish
		"odeslaná pošta", "odeslané", "trimise", // Czech, Romanian
	}
	ImapDrafts = []string{
		"draft", "drafts",
		"borradores", "brouillons", "entwürfe", "concepten",
		"rascunhos", "bozze", "utkast", "luonnokset",
		"piszkozatok", "kopie robocze", "koncepty", "rozepsané", "ciorne",
	}
	ImapTrash = []string{
		"trash", "bin", "deleted", "deleted items", "deleted messages",
		"papelera", "elementos eliminados", // Spanish
		"corbeille", "éléments supprimés", // French
		"papierkorb", "gelöschte elemente", "gelöschte objekte", // German
		"prullenbak", "verwijderde items", // Dutch
		"lixeira", "itens excluídos", // Portuguese
		"cestino", "posta eliminata", // Italian
		"papperskorgen", "slettet post", "roskakori", // Nordic
		"törölt elemek", "kuka", // Hungarian
		"kosz", "elementy usunięte", // Polish
		"koš", "coș de gunoi", // Czech, Romanian
	}
	ImapArchive = []string{
		"archive", "archives", "all mail",
		"archivo", "archivado", "archives", "archiv",
		"archief", "arquivo", "archivio", "arkiv",
		"arkisto", "archívum", "archiwum", "arhiva",
	}
)

func hasPrefixFold(a, b string) bool {
	return strings.HasPrefix(strings.ToLower(a), strings.ToLower(b))
}
func leafWithDelim(name, delim string) string {
	if delim != "" {
		if i := strings.LastIndex(name, delim); i >= 0 {
			return name[i+len(delim):]
		}
	}
	return name
}

func IsArchiveMailbox(name string, attrs []string) bool {
	for _, a := range attrs {
		switch strings.ToLower(a) {
		case "\\archive", "\\all":
			return true
		}
	}
	return matchesFolderName(strings.ToLower(leaf(strings.TrimSpace(name))), ImapArchive)
}

// IsSentMailbox returns true if the mailbox's attributes or name identify it as
// the Sent folder, under RFC 6154 SPECIAL-USE or by name match.
func IsSentMailbox(name string, attrs []string) bool {
	for _, a := range attrs {
		if strings.EqualFold(a, "\\Sent") {
			return true
		}
	}
	return matchesFolderName(strings.ToLower(leaf(strings.TrimSpace(name))), ImapSent)
}

// IsInboxMailbox returns true for the canonical INBOX (case-insensitive) or
// any mailbox flagged with the \Inbox special-use attribute.
func IsInboxMailbox(name string, attrs []string) bool {
	if strings.EqualFold(strings.TrimSpace(name), "INBOX") {
		return true
	}
	for _, a := range attrs {
		if strings.EqualFold(a, "\\Inbox") {
			return true
		}
	}
	return false
}

func leaf(name string) string {
	if i := strings.LastIndexAny(name, "./"); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}
