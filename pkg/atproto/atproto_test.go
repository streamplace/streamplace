package atproto

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/bluesky-social/indigo/xrpc"
)

func TestKeyResolution(t *testing.T) {
	// dir, err := os.MkdirTemp("", "atproto-test-*")
	// require.NoError(t, err)
	// defer os.RemoveAll(dir)

	// fname := filepath.Join(dir, "db.sqlite")
	// mod, err := model.MakeDB(fname)
	// require.NoError(t, err)
	// oldResolveIdent := ResolveIdent
	// ResolveIdent = func(ctx context.Context, arg string) (*identity.Identity, error) {
	// 	var doc identity.DIDDocument
	// 	err = json.Unmarshal(didDoc, &doc)
	// 	require.NoError(t, err)

	// 	id := identity.ParseIdentity(&doc)
	// 	return &id, nil
	// }
	// defer func() { ResolveIdent = oldResolveIdent }()
	// oldSyncGetRepo := SyncGetRepo
	// defer func() { SyncGetRepo = oldSyncGetRepo }()

	// atsync := &ATProtoSynchronizer{
	// 	Model: mod,
	// }

	// ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{
	// 	"func": {"handleCreateUpdate": 9, "SyncBlueskyRepo": 9},
	// })

	// // full sync
	// SyncGetRepo = MockSyncGetRepo(fullSync)
	// repo, err := atsync.SyncBlueskyRepo(ctx, "streamplace-test", mod)
	// require.NoError(t, err)
	// keys, err := mod.GetSigningKeysForRepo(repo.DID)
	// require.NoError(t, err)
	// require.Len(t, keys, 1)
	// require.Equal(t, firstKey, keys[0].DID)

	// // incremental sync with no changes
	// SyncGetRepo = MockSyncGetRepo(incrementalSyncSameKey)
	// repo, err = atsync.SyncBlueskyRepo(context.Background(), "streamplace-test", mod)
	// require.NoError(t, err)
	// keys, err = mod.GetSigningKeysForRepo(repo.DID)
	// require.NoError(t, err)
	// require.Len(t, keys, 1)
	// require.Equal(t, firstKey, keys[0].DID)

	// // incremental sync with a new streamplace key
	// SyncGetRepo = MockSyncGetRepo(incrementalSyncNewKey)
	// repo, err = atsync.SyncBlueskyRepo(context.Background(), "streamplace-test", mod)
	// require.NoError(t, err)
	// keys, err = mod.GetSigningKeysForRepo(repo.DID)
	// require.NoError(t, err)
	// require.Len(t, keys, 2)
	// require.Equal(t, firstKey, keys[0].DID)
	// require.Equal(t, secondKey, keys[1].DID)

	// // empty sync
	// SyncGetRepo = MockSyncGetRepo(emptySync)
	// repo, err = atsync.SyncBlueskyRepo(context.Background(), "streamplace-test", mod)
	// require.NoError(t, err)
	// keys, err = mod.GetSigningKeysForRepo(repo.DID)
	// require.NoError(t, err)
	// require.Len(t, keys, 2)
	// require.Equal(t, firstKey, keys[0].DID)
	// require.Equal(t, secondKey, keys[1].DID)
}

func MockSyncGetRepo(res string) func(ctx context.Context, xrpcc *xrpc.Client, did string, rev string) ([]byte, error) {
	return func(ctx context.Context, xrpcc *xrpc.Client, did string, rev string) ([]byte, error) {
		decoded, err := base64.URLEncoding.DecodeString(res)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}
}

// captured from plc.directory
var didDoc = []byte(`
	{
		"@context": [
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
			"https://w3id.org/security/suites/secp256k1-2019/v1"
		],
		"alsoKnownAs": [
			"at://streamplace-test.bsky.social"
		],
		"id": "did:plc:ee3n2hs2wkgrkskrz6whzrfs",
		"service": [
			{
				"id": "#atproto_pds",
				"serviceEndpoint": "https://grisette.us-west.host.bsky.network",
				"type": "AtprotoPersonalDataServer"
			}
		],
		"verificationMethod": [
			{
				"controller": "did:plc:ee3n2hs2wkgrkskrz6whzrfs",
				"id": "did:plc:ee3n2hs2wkgrkskrz6whzrfs#atproto",
				"publicKeyMultibase": "zQ3shYTpdwFJJppCFwncKrB1hSTsKE48s1kTQASKvgSNm3jTt",
				"type": "Multikey"
			}
		]
	}
`)
var firstKey = "did:key:zQ3shkP7pgLBqp7PYQYnFRSCTBdKAGHWKKghF3GidVmEMUrct"
var secondKey = "did:key:zQ3shUjidTbELwdHjeNyvx8iQ8adi4789Eaan9Lgc9AUh1Vzm"
var fullSync = `OqJlcm9vdHOB2CpYJQABcRIgMDc7EDUmj3o8uSuYkHfG46c34xUOpb9gmVTu-M9CbNdndmVyc2lvbgGmAQFxEiCHwNkrZ3dGh8V43q6FFuRg5LlgE50hifxzDmAdATYc6aNlJHR5cGVwcGxhY2Uuc3RyZWFtLmtleWljcmVhdGVkQXR4GDIwMjUtMDEtMjZUMjE6MDc6MzkuMTYxWmpzaWduaW5nS2V5eDlkaWQ6a2V5OnpRM3Noa1A3cGdMQnFwN1BZUVluRlJTQ1RCZEtBR0hXS0tnaEYzR2lkVm1FTVVyY3T4AQFxEiBzaFlz2gS4XkDfdJHIjomgqdCpk7Cc49rX3kvYqLP6eaJhZYKkYWtYG2FwcC5ic2t5LmFjdG9yLnByb2ZpbGUvc2VsZmFwAGF02CpYJQABcRIgVDDzvM6e19so-zvPpJEldfpzAbpZZ6WxE81LAYvjCv1hdtgqWCUAAXESIMzJIP4DaRsw1NyMu1Su2ggAXMDz0nOK13rNPkQz00L0pGFrWB5wbGFjZS5zdHJlYW0ua2V5LzNsZ29kZ3Q2bzY1MjRhcABhdPZhdtgqWCUAAXESIIfA2Stnd0aHxXjeroUW5GDkuWATnSGJ_HMOYB0BNhzpYWz24AEBcRIgMDc7EDUmj3o8uSuYkHfG46c34xUOpb9gmVTu-M9CbNemY2RpZHggZGlkOnBsYzplZTNuMmhzMndrZ3Jrc2tyejZ3aHpyZnNjcmV2bTNsZ29kZ3Q2dHpuMjRjc2lnWEDo6aLAaagS8QCgO_fuXpYApipc9VpFI4mmOPpmIIKWyHQnXiMurYyqNs0oGX31lXUmBlNQq0mUDKY8D2FbxSyaZGRhdGHYKlglAAFxEiBzaFlz2gS4XkDfdJHIjomgqdCpk7Cc49rX3kvYqLP6eWRwcmV29md2ZXJzaW9uA84BAXESIMzJIP4DaRsw1NyMu1Su2ggAXMDz0nOK13rNPkQz00L0pGUkdHlwZXZhcHAuYnNreS5hY3Rvci5wcm9maWxlZmF2YXRhcqRjcmVm2CpYJQABVRIgHdCQPIz2IErjjii59bHG-XOJjtGVJLiR1RRnLKsXK3xkc2l6ZRl3ZmUkdHlwZWRibG9iaG1pbWVUeXBlaWltYWdlL3BuZ2ljcmVhdGVkQXR4GDIwMjUtMDEtMjZUMjE6MDQ6MzUuNjMxWmtkaXNwbGF5TmFtZWCEAQFxEiBUMPO8zp7X2yj7O8-kkSV1-nMBullnpbETzUsBi-MK_aJhZYGkYWtYI2FwcC5ic2t5LmdyYXBoLmZvbGxvdy8zbGdvZGJkM3poYzJoYXAAYXT2YXbYKlglAAFxEiAOO157S10FbyFkhacvqpz-lvrDDv1i9kTTw7p26_NFX2Fs9o8BAXESIA47XntLXQVvIWSFpy-qnP6W-sMO_WL2RNPDunbr80Vfo2UkdHlwZXVhcHAuYnNreS5ncmFwaC5mb2xsb3dnc3ViamVjdHggZGlkOnBsYzp6NzJpN2hkeW5tazZyMjJ6MjdoNnR2dXJpY3JlYXRlZEF0eBgyMDI1LTAxLTI2VDIxOjA0OjM0LjcxM1o=`
var incrementalSyncSameKey = `OqJlcm9vdHOB2CpYJQABcRIgKRCUWJjPxaq1aqO7srwRxpJeURCsoQ_sXJaSkyscKXRndmVyc2lvbgHRAQFxEiD8HmkbON-0uUXvBD5PHbTu9hfJYYh7BcQBBbua6OlXNqJhZYKkYWtYIGFwcC5ic2t5LmZlZWQucG9zdC8zbGdvZGx3eGg0dDJoYXAAYXT2YXbYKlglAAFxEiAJRIEWBgnKqUlv19Ese8UO557t5YL7jfcEBItUpEm9IqRha1gaZ3JhcGguZm9sbG93LzNsZ29kYmQzemhjMmhhcAlhdPZhdtgqWCUAAXESIA47XntLXQVvIWSFpy-qnP6W-sMO_WL2RNPDunbr80VfYWz2-AEBcRIguxQZbYxqjerYzPJzsB2cq47_Cy-M9DbdfX3zgsZsHSKiYWWCpGFrWBthcHAuYnNreS5hY3Rvci5wcm9maWxlL3NlbGZhcABhdNgqWCUAAXESIPweaRs437S5Re8EPk8dtO72F8lhiHsFxAEFu5ro6Vc2YXbYKlglAAFxEiDMySD-A2kbMNTcjLtUrtoIAFzA89Jzitd6zT5EM9NC9KRha1gecGxhY2Uuc3RyZWFtLmtleS8zbGdvZGd0Nm82NTI0YXAAYXT2YXbYKlglAAFxEiCHwNkrZ3dGh8V43q6FFuRg5LlgE50hifxzDmAdATYc6WFs9uABAXESICkQlFiYz8WqtWqju7K8EcaSXlEQrKEP7FyWkpMrHCl0pmNkaWR4IGRpZDpwbGM6ZWUzbjJoczJ3a2dya3Nrcno2d2h6cmZzY3Jldm0zbGdvZGx3eXhkeDJqY3NpZ1hAokbAmjkgNdLlufRil-TeFuxKggit3Ljqkbw7QKRY794EPZYnb1WQE6sPhcF7Z0VNTl05APJPEb0s4SKM8qloRGRkYXRh2CpYJQABcRIguxQZbYxqjerYzPJzsB2cq47_Cy-M9DbdfX3zgsZsHSJkcHJldvZndmVyc2lvbgOIAQFxEiAJRIEWBgnKqUlv19Ese8UO557t5YL7jfcEBItUpEm9IqRkdGV4dHZIaSB0aGlzIGlzIGEgdGVzdCBwb3N0ZSR0eXBlcmFwcC5ic2t5LmZlZWQucG9zdGVsYW5nc4FiZW5pY3JlYXRlZEF0eBgyMDI1LTAxLTI2VDIxOjEwOjMxLjA4MFo=`
var incrementalSyncNewKey = `OqJlcm9vdHOB2CpYJQABcRIglIEJIf3uyrHn7zpaDG0VZ5LFVpEnAO4COXPLdZVI5N5ndmVyc2lvbgGmAQFxEiC7UEFQdgwPLi4Aph-E0JNXfiJhpy7EK7yIae9mt-JLlqNlJHR5cGVwcGxhY2Uuc3RyZWFtLmtleWljcmVhdGVkQXR4GDIwMjUtMDEtMjZUMjE6MTA6NTkuMDI5WmpzaWduaW5nS2V5eDlkaWQ6a2V5OnpRM3NoVWppZFRiRUx3ZEhqZU55dng4aVE4YWRpNDc4OUVhYW45TGdjOUFVaDFWem3gAQFxEiCUgQkh_e7KsefvOloMbRVnksVWkScA7gI5c8t1lUjk3qZjZGlkeCBkaWQ6cGxjOmVlM24yaHMyd2tncmtza3J6NndoenJmc2NyZXZtM2xnb2RtcnJqb3EydGNzaWdYQPoUwYR8ZFqHPygnPU0Yzo14pRo35huNbjVwtQudrSIMErQMHsGhZL-JdLnq5avgxXYqq7k4jcMUEkUIgmZFI51kZGF0YdgqWCUAAXESIEBLuaWzxeuRKjP8Kovw6ZWkkvqfMkhDOBymBuE4ZZhRZHByZXb2Z3ZlcnNpb24DtQIBcRIgQEu5pbPF65EqM_wqi_DplaSS-p8ySEM4HKYG4ThlmFGiYWWDpGFrWBthcHAuYnNreS5hY3Rvci5wcm9maWxlL3NlbGZhcABhdNgqWCUAAXESIPweaRs437S5Re8EPk8dtO72F8lhiHsFxAEFu5ro6Vc2YXbYKlglAAFxEiDMySD-A2kbMNTcjLtUrtoIAFzA89Jzitd6zT5EM9NC9KRha1gecGxhY2Uuc3RyZWFtLmtleS8zbGdvZGd0Nm82NTI0YXAAYXT2YXbYKlglAAFxEiCHwNkrZ3dGh8V43q6FFuRg5LlgE50hifxzDmAdATYc6aRha0htcnJmcnEydGFwFmF09mF22CpYJQABcRIgu1BBUHYMDy4uAKYfhNCTV34iYacuxCu8iGnvZrfiS5ZhbPY=`
var emptySync = `OqJlcm9vdHOB2CpYJQABcRIglIEJIf3uyrHn7zpaDG0VZ5LFVpEnAO4COXPLdZVI5N5ndmVyc2lvbgE=`
