package pramaan

import (
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
)

var ErrReservedRecipient = errors.New("Email was not sent: use a real recipient or an approved test inbox, not a reserved address.")

// ValidateMailRecipients runs before any external call, including consent lookups.
func ValidateMailRecipients(groups ...[]string) error {
	for _, group := range groups {
		for _, value := range group {
			address, err := mail.ParseAddress(value)
			if err != nil {
				return errors.New("Email was not sent: a recipient address is invalid.")
			}
			parts := strings.Split(address.Address, "@")
			if len(parts) != 2 {
				return ErrReservedRecipient
			}
			domain := strings.TrimSuffix(strings.ToLower(parts[1]), ".")
			if !strings.Contains(domain, ".") {
				return ErrReservedRecipient
			}
			for _, reserved := range []string{"test", "invalid", "localhost", "local", "example", "example.com", "example.net", "example.org"} {
				if domain == reserved || strings.HasSuffix(domain, "."+reserved) {
					return ErrReservedRecipient
				}
			}
		}
	}
	return nil
}

func validateMailInput(id string, input any) error {
	switch id {
	case "email.send", "email.message.send", "email.message.reply", "email.send-bulk":
	default:
		return nil
	}
	b, err := json.Marshal(input)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err = json.Unmarshal(b, &obj); err != nil {
		return err
	}
	if id == "email.send-bulk" {
		if rows, ok := obj["rows"].([]any); ok {
			for _, row := range rows {
				if err := validateMailInput("email.send", row); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if raw, ok := obj["headers"]; ok {
		headers := map[string]string{}
		encoded, _ := json.Marshal(raw)
		if json.Unmarshal(encoded, &headers) != nil {
			return errors.New("Message headers must be text values.")
		}
		if err := ValidateMailHeaders(headers); err != nil {
			return err
		}
		if len(headers) > 0 && id == "email.send" && obj["sender"] != "domain" {
			return errors.New("Custom headers require an owned-domain sender.")
		}
		if len(headers) > 0 && id == "email.message.reply" {
			return errors.New("Custom reply headers are unavailable.")
		}
	}
	for _, key := range []string{"to", "cc", "bcc", "replyTo"} {
		value, exists := obj[key]
		if !exists || value == nil {
			continue
		}
		var values []string
		switch v := value.(type) {
		case string:
			if key == "replyTo" && v == "" {
				continue
			}
			values = []string{v}
		case []any:
			for _, a := range v {
				s, ok := a.(string)
				if !ok {
					return ErrReservedRecipient
				}
				values = append(values, s)
			}
		default:
			return ErrReservedRecipient
		}
		if err := ValidateMailRecipients(values); err != nil {
			return err
		}
	}
	return nil
}
