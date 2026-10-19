// Package push sends real Firebase Cloud Messaging notifications to
// registered customer/employee devices. Genuine integration, not a mock:
// it authenticates against Firebase using a real service account and
// calls the real FCM send API — it just has no effect (Send/SendToUser
// become no-ops, logged once) when no credential is configured, exactly
// the same "real code, inert without live credentials" pattern
// internal/payments/razorpay.go already uses.
package push

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	initOnce   sync.Once
	client     *messaging.Client
	warnedOnce sync.Once
)

// FirebaseCredentialParam is the SSM Parameter Store name holding the raw
// Firebase service account JSON — deliberately NOT an Elastic Beanstalk
// environment variable. Two real, confirmed problems with putting it
// there: (1) EB's environment-variable injection corrupts a raw
// multi-line PEM private key — every literal `\n` inside the value
// silently loses its backslash and becomes a bare `n` by the time the
// process sees it (confirmed directly via /proc/<pid>/environ: the EB
// config API itself showed the correct value, only the actual process
// environment was affected), which breaks the PEM's ASN.1 structure and
// made every real push send fail with an opaque "private key should be a
// PEM or plain PKCS1 or PKCS8" parse error no matter how carefully the
// JSON was set or re-set. (2) Base64-encoding it to dodge that then hit
// a second, harder wall: EB's CloudFormation template caps the combined
// size of ALL environment variables together at 4096 characters, and
// base64's ~33% size overhead pushed the total over that limit outright
// ("Parameter 'EnvironmentVariables' ... length is greater than 4096"),
// failing the deploy entirely. SSM Parameter Store has no such limit and
// isn't subject to EB's env-var injection pipeline at all.
var FirebaseCredentialParam = "/nbfc/firebase-service-account-json"

// initClient lazily builds the FCM client, fetching the service account
// JSON from SSM Parameter Store (production) or the
// FIREBASE_SERVICE_ACCOUNT_JSON env var (local dev convenience — not
// subject to EB's env-var pipeline when running outside EB).
func initClient() {
	initOnce.Do(func() {
		credentialJSON := fetchCredentialJSON()
		if len(credentialJSON) == 0 {
			return
		}
		app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsJSON(credentialJSON))
		if err != nil {
			log.Println("push: failed to initialize Firebase app:", err)
			return
		}
		c, err := app.Messaging(context.Background())
		if err != nil {
			log.Println("push: failed to initialize FCM client:", err)
			return
		}
		client = c
	})
}

func fetchCredentialJSON() []byte {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithEC2IMDSRegion())
	if err == nil {
		out, err := ssm.NewFromConfig(cfg).GetParameter(context.Background(), &ssm.GetParameterInput{
			Name:           &FirebaseCredentialParam,
			WithDecryption: boolPtr(true),
		})
		if err == nil && out.Parameter != nil && out.Parameter.Value != nil && *out.Parameter.Value != "" {
			return []byte(*out.Parameter.Value)
		}
		if err != nil {
			log.Println("push: could not read Firebase credential from SSM Parameter Store (falling back to env var):", err)
		}
	}
	raw := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))
	if raw == "" {
		return nil
	}
	return []byte(raw)
}

func boolPtr(b bool) *bool { return &b }

// Send delivers one push to one device token. Best-effort: an invalid or
// expired token is logged, not returned as a caller-visible error — the
// in-app Notification row (see internal/notifications.Create) is always
// the durable record regardless of whether the push itself lands.
func Send(token, title, body string) {
	initClient()
	if client == nil {
		warnedOnce.Do(func() {
			log.Println("push: FIREBASE_SERVICE_ACCOUNT_JSON not configured — push notifications disabled, in-app notifications still work")
		})
		return
	}
	_, err := client.Send(context.Background(), &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		// Without an explicit AndroidConfig, FCM defaults Android delivery
		// to "normal" priority — which the OS is free to delay/batch
		// while the device is idle or the app is backgrounded, exactly
		// the "only arrives when the app is already open" symptom. High
		// priority tells FCM/Play Services to wake the device and
		// deliver immediately regardless of Doze state. ChannelId must
		// match the channel NotificationService actually creates
		// (internal/core/services/notification_service.dart) or some
		// OEM's system UI silently drops the notification instead of
		// falling back to a default channel.
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "onefinance_general",
			},
		},
	})
	if err != nil {
		log.Println("push: send failed for token ending ..."+lastChars(token, 6)+":", err)
	}
}

// SendToUser pushes to every device this customer/employee has ever
// registered a token from (internal/push.RegisterTokenHandler) — not
// just their most recent device, since a customer may have the app
// installed on more than one phone.
func SendToUser(role, userID, title, body string) {
	var tokens []models.DeviceToken
	database.DB.Where("role = ? AND user_id = ?", role, userID).Find(&tokens)
	for _, t := range tokens {
		Send(t.Token, title, body)
	}
}

func lastChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
