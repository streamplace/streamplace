package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/streamplace"
)

func TestWebhookCreation(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		webhook := &Webhook{
			UserDID: "did:web:example.com",
			URL:     "https://example.com",
			Events:  []byte(`["test"]`),
			Active:  true,
		}
		err := state.CreateWebhook(webhook)
		require.NoError(t, err)
		activeWebhooks, err := state.GetActiveWebhooksForUser("did:web:example.com", "test")
		require.NoError(t, err)
		require.Len(t, activeWebhooks, 1)
		require.Equal(t, webhook.URL, activeWebhooks[0].URL)
		require.Equal(t, webhook.Events, activeWebhooks[0].Events)
	})
}

func TestWebhookUpdatePartially(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		webhook := &Webhook{
			ID:      "webhook1",
			UserDID: "did:web:example.com",
			URL:     "https://example.com",
			Events:  []byte(`["test"]`),
		}
		err := state.CreateWebhook(webhook)
		require.NoError(t, err)

		// Update only the URL
		webhook.URL = "https://newexample.com"
		updatedWebhook, err := state.UpdateWebhook(webhook.ID, webhook.UserDID,
			map[string]any{
				"url": webhook.URL,
			},
		)
		require.NoError(t, err)

		require.NoError(t, err)
		require.Equal(t, "https://newexample.com", updatedWebhook.URL)
		require.JSONEq(t, `["test"]`, string(updatedWebhook.Events))
	})
}

func TestWebhookCrossUserAccess(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		webhook := &Webhook{
			ID:      "webhook2",
			UserDID: "did:web:example.com",
			URL:     "https://example.com",
			Events:  []byte(`["test"]`),
		}
		err := state.CreateWebhook(webhook)
		require.NoError(t, err)

		// malicious user tries to update
		_, err = state.UpdateWebhook(webhook.ID, "did:web:not.example.com",
			map[string]any{
				"url": "https://malicious.com",
			},
		)
		require.Error(t, err)

		// malicious user tries to delete
		err = state.DeleteWebhook(webhook.ID, "did:web:not.example.com")
		require.Error(t, err)
	})
}

func TestWebhookDelete(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		webhook := &Webhook{
			ID:      "webhook3",
			UserDID: "did:web:example.com",
			URL:     "https://example.com",
			Events:  []byte(`["test"]`),
		}
		err := state.CreateWebhook(webhook)
		require.NoError(t, err)

		err = state.DeleteWebhook(webhook.ID, webhook.UserDID)
		require.NoError(t, err)
		_, err = state.GetWebhook(webhook.ID, webhook.UserDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "webhook not found")
	})
}

func TestWebhookListPaginationAndFilters(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		// clear for clean tests
		state.DB.Where("1 = 1").Delete(&Webhook{})

		for i := 1; i <= 5; i++ {
			webhook := &Webhook{
				UserDID: "did:web:example.com",
				URL:     "https://example.com/" + string(rune(i)),
				Events:  []byte(`["test"]`),
				Active:  i%2 == 0,
			}
			t.Logf("webhook should be %v", webhook.Active)
			err := state.CreateWebhook(webhook)
			require.NoError(t, err)

			// Debug: verify what was actually saved
			savedWebhook, err := state.GetWebhook(webhook.ID, webhook.UserDID)
			require.NoError(t, err)
			t.Logf("Webhook %d: expected active=%v, saved active=%v", i, webhook.Active, savedWebhook.Active)
		}

		// with paging
		webhooks, err := state.ListWebhooks("did:web:example.com", 2, 1, map[string]any{})
		require.NoError(t, err)
		require.Len(t, webhooks, 2)

		// with filter
		activeWebhooks, err := state.ListWebhooks("did:web:example.com", 5, 0, map[string]any{"active": true})
		require.NoError(t, err)
		require.Len(t, activeWebhooks, 2)
	})
}

func TestWebhookCreateEmptyURLFails(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		webhook := &Webhook{UserDID: "did:web:example.com", URL: "", Events: []byte(`["test"]`)}
		err := state.CreateWebhook(webhook)
		require.Error(t, err)
	})
}

func TestWebhookConversionFunctions(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		// Test FromLexiconInput conversion
		active := true
		prefix := "test-prefix"
		suffix := "test-suffix"
		name := "test-name"
		description := "test description"

		input := &streamplace.ServerCreateWebhook_Input{
			Url:         "https://example.com/webhook",
			Events:      []string{"chat", "livestream"},
			Active:      &active,
			Prefix:      &prefix,
			Suffix:      &suffix,
			Name:        &name,
			Description: &description,
			Rewrite: []*streamplace.ServerDefs_RewriteRule{
				{From: "old", To: "new"},
				{From: "hello", To: "hi"},
			},
		}

		webhook, err := WebhookFromLexiconInput(input, "did:web:example.com", "test-webhook-id")
		require.NoError(t, err)
		require.Equal(t, "test-webhook-id", webhook.ID)
		require.Equal(t, "did:web:example.com", webhook.UserDID)
		require.Equal(t, "https://example.com/webhook", webhook.URL)
		require.True(t, webhook.Active)
		require.Equal(t, "test-prefix", webhook.Prefix)
		require.Equal(t, "test-suffix", webhook.Suffix)
		require.Equal(t, "test-name", webhook.Name)
		require.Equal(t, "test description", webhook.Description)

		// Save to database and retrieve
		err = state.CreateWebhook(webhook)
		require.NoError(t, err)

		retrievedWebhook, err := state.GetWebhook("test-webhook-id", "did:web:example.com")
		require.NoError(t, err)

		// Test ToLexicon conversion
		lexiconWebhook, err := retrievedWebhook.ToLexicon()
		require.NoError(t, err)
		require.Equal(t, "test-webhook-id", lexiconWebhook.Id)
		require.Equal(t, "https://example.com/webhook", lexiconWebhook.Url)
		require.True(t, lexiconWebhook.Active)
		require.Equal(t, []string{"chat", "livestream"}, lexiconWebhook.Events)
		require.NotNil(t, lexiconWebhook.Prefix)
		require.Equal(t, "test-prefix", *lexiconWebhook.Prefix)
		require.NotNil(t, lexiconWebhook.Suffix)
		require.Equal(t, "test-suffix", *lexiconWebhook.Suffix)
		require.NotNil(t, lexiconWebhook.Name)
		require.Equal(t, "test-name", *lexiconWebhook.Name)
		require.NotNil(t, lexiconWebhook.Description)
		require.Equal(t, "test description", *lexiconWebhook.Description)
		require.Len(t, lexiconWebhook.Rewrite, 2)
		require.Equal(t, "old", lexiconWebhook.Rewrite[0].From)
		require.Equal(t, "new", lexiconWebhook.Rewrite[0].To)
		require.Equal(t, "hello", lexiconWebhook.Rewrite[1].From)
		require.Equal(t, "hi", lexiconWebhook.Rewrite[1].To)
	})
}
