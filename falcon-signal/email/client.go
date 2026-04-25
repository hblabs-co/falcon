package email

import (
	"fmt"
	"strings"

	mailjet "github.com/mailjet/mailjet-apiv3-go/v4"
	"hblabs.co/falcon/packages/system"
)

// Client wraps the Mailjet API client.
type Client struct {
	mj *mailjet.Client
}

// NewClient creates a new email client.
func NewClient() *Client {
	apiKey := system.MustEnv("MAILJET_API_KEY")
	secretKey := system.MustEnv("MAILJET_SECRET_KEY")
	return &Client{mj: mailjet.NewMailjetClient(apiKey, secretKey)}
}

// Send delivers an email using a named template, language, and per-email variables.
func (c *Client) Send(toEmail, templateName, lang string, vars map[string]string) error {
	t := T(templateName, lang)

	html, err := Render(templateName, lang, vars)
	if err != nil {
		return err
	}

	textBody := t["text_body"]
	if link, ok := vars["magic_link"]; ok && strings.Contains(textBody, "%s") {
		textBody = fmt.Sprintf(textBody, link)
	}

	messages := mailjet.MessagesV31{
		Info: []mailjet.InfoMessagesV31{
			{
				From: &mailjet.RecipientV31{
					Email: cfg.Shared["sender_email"],
					Name:  cfg.Shared["sender_name"],
				},
				To: &mailjet.RecipientsV31{
					{Email: toEmail},
				},
				Subject:  t["subject"],
				HTMLPart: html,
				TextPart: textBody,
			},
		},
	}

	res, err := c.mj.SendMailV31(&messages)
	if err != nil {
		return fmt.Errorf("mailjet send: %w", err)
	}
	if len(res.ResultsV31) > 0 && res.ResultsV31[0].Status != "success" {
		return fmt.Errorf("mailjet delivery status: %s", res.ResultsV31[0].Status)
	}
	return nil
}

// SendMagicLink is a convenience wrapper for the login template.
func (c *Client) SendMagicLink(toEmail, magicLink, lang string) error {
	return c.Send(toEmail, "login", lang, map[string]string{
		"magic_link": magicLink,
	})
}
