package signal

import (
	_ "embed"
	"fmt"
	"strings"

	mailjet "github.com/mailjet/mailjet-apiv3-go/v4"
	"hblabs.co/falcon/common/system"
)

//go:embed logo_base64.txt
var logoBase64Raw string

const senderEmail = "start@falcon.hblabs.co"
const senderName = "Falcon Team"

type mailjetClient struct {
	mj *mailjet.Client
}

func newMailjetClient() *mailjetClient {
	apiKey := system.MustEnv("MAILJET_API_KEY")
	secretKey := system.MustEnv("MAILJET_SECRET_KEY")
	return &mailjetClient{mj: mailjet.NewMailjetClient(apiKey, secretKey)}
}

func (m *mailjetClient) SendMagicLink(toEmail, magicLink string) error {
	messages := mailjet.MessagesV31{
		Info: []mailjet.InfoMessagesV31{
			{
				From: &mailjet.RecipientV31{
					Email: senderEmail,
					Name:  senderName,
				},
				To: &mailjet.RecipientsV31{
					{Email: toEmail},
				},
				Subject:  "Your Falcon login link",
				HTMLPart: buildMagicLinkHTML(magicLink),
				TextPart: "Log in to Falcon\n\nTap the link below to access your account:\n" + magicLink + "\n\nThis link expires in 15 minutes and can only be used once.\nIf you didn't request this, just ignore this email.",
			},
		},
	}

	res, err := m.mj.SendMailV31(&messages)
	if err != nil {
		return fmt.Errorf("mailjet send: %w", err)
	}
	if len(res.ResultsV31) > 0 && res.ResultsV31[0].Status != "success" {
		return fmt.Errorf("mailjet delivery status: %s", res.ResultsV31[0].Status)
	}
	return nil
}

func logoDataURI() string {
	return "data:image/png;base64," + strings.TrimSpace(logoBase64Raw)
}

func buildMagicLinkHTML(magicLink string) string {
	logo := logoDataURI()
	return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f2f2f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Helvetica Neue',Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="padding:48px 16px;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:20px;overflow:hidden;box-shadow:0 8px 40px rgba(0,0,0,.08);">

        <!-- Logo header -->
        <tr>
          <td align="center" style="padding:36px 40px 0;">
            <img src="` + logo + `" alt="Falcon" width="56" height="56" style="display:block;border-radius:14px;">
          </td>
        </tr>

        <!-- Title -->
        <tr>
          <td align="center" style="padding:20px 40px 0;">
            <h1 style="margin:0;font-size:24px;font-weight:700;color:#1d1d1f;letter-spacing:-0.3px;">Log in to Falcon</h1>
          </td>
        </tr>

        <!-- Body text -->
        <tr>
          <td align="center" style="padding:12px 40px 0;">
            <p style="margin:0;font-size:15px;line-height:22px;color:#6e6e73;">
              Tap the button below to securely access your account.<br>
              This link expires in <strong>15 minutes</strong> and can only be used once.
            </p>
          </td>
        </tr>

        <!-- CTA Button -->
        <tr>
          <td align="center" style="padding:32px 40px;">
            <a href="` + magicLink + `" target="_blank" style="display:inline-block;background:#0071e3;color:#ffffff;font-size:16px;font-weight:600;text-decoration:none;padding:14px 40px;border-radius:14px;letter-spacing:0.2px;">
              Log in
            </a>
          </td>
        </tr>

        <!-- Divider -->
        <tr>
          <td style="padding:0 40px;">
            <hr style="border:none;border-top:1px solid #e5e5ea;margin:0;">
          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td align="center" style="padding:20px 40px 32px;">
            <p style="margin:0;font-size:12px;line-height:18px;color:#aeaeb2;">
              If you didn't request this email, you can safely ignore it.<br>
              &copy; Falcon &middot; hblabs.co
            </p>
          </td>
        </tr>

      </table>
    </td></tr>
  </table>
</body>
</html>`
}
