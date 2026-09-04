package config

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"math/rand/v2"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	urfavecli "github.com/urfave/cli/v3"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/moderation"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/s3"
)

const SPDataDir = "$SP_DATA_DIR"
const SegmentsDir = "segments"
const ThumbnailsDir = "thumbnails"

type BuildFlags struct {
	Version   string
	BuildTime int64
	UUID      string
}

func (b BuildFlags) BuildTimeStr() string {
	ts := time.Unix(b.BuildTime, 0)
	return ts.UTC().Format(time.RFC3339)
}

func (b BuildFlags) BuildTimeStrExpo() string {
	ts := time.Unix(b.BuildTime, 0)
	return ts.UTC().Format("2006-01-02T15:04:05.000Z")
}

type CLI struct {
	AdminAccount                string
	Build                       *BuildFlags
	DataDir                     string
	DBURL                       string
	LocalDBURL                  string
	EthAccountAddr              string
	EthKeystorePath             string
	EthPassword                 string
	FirebaseServiceAccount      string
	FirebaseServiceAccountFile  string
	GitLabURL                   string
	HTTPAddr                    string
	HTTPInternalAddr            string
	HTTPSAddr                   string
	RTMPAddr                    string
	RTMPSAddr                   string
	RTMPSAddonAddr              string
	Secure                      bool
	NoMist                      bool
	LLHLS                       bool
	IsolatedIngest              bool
	MistAdminPort               int
	MistHTTPPort                int
	MistRTMPPort                int
	SigningKeyPath              string
	TAURL                       string
	TLSCertPath                 string
	TLSKeyPath                  string
	PKCS11ModulePath            string
	PKCS11Pin                   string
	PKCS11TokenSlot             string
	PKCS11TokenLabel            string
	PKCS11TokenSerial           string
	PKCS11KeypairLabel          string
	PKCS11KeypairID             string
	StreamerName                string
	RelayHost                   string
	Debug                       map[string]map[string]int
	AllowedStreams              []string
	WideOpen                    bool
	Peers                       []string
	Redirects                   map[string]string
	TestStream                  bool
	FrontendProxy               string
	Frontend                    string
	PublicOAuth                 bool
	AppBundleID                 string
	NoFirehose                  bool
	PrintChat                   bool
	Color                       string
	LivepeerGatewayURL          string
	LivepeerGateway             bool
	WHIPTest                    string
	Thumbnail                   bool
	ExternalSigning             bool
	RTMPServerAddon             string
	TracingEndpoint             string
	BroadcasterHost             string
	XXDeprecatedPublicHost      string
	ServerHost                  string
	RateLimitPerSecond          int
	RateLimitBurst              int
	RateLimitWebsocket          int
	JWK                         jwk.Key
	AccessJWK                   jwk.Key
	ServiceAuthKey              jwk.Key
	dataDirFlags                []*string
	DiscordWebhooks             []*discordtypes.Webhook
	AppleTeamID                 string
	AndroidCertFingerprint      string
	Labelers                    []string
	AtprotoDID                  string
	LivepeerHelp                bool
	PLCURL                      string
	ContentFilters              *ContentFilters
	ModerationDir               string
	DefaultRecommendedStreamers []string
	SQLLogging                  bool
	SentryDSN                   string
	LivepeerDebug               bool
	Tickets                     []string
	IrohTopic                   string
	DID                         string
	DisableIrohRelay            bool
	DevAccountCreds             map[string]string
	StreamSessionTimeout        time.Duration
	LegacySegmentCleaner        bool
	SegmentArchiveRetention     time.Duration
	Replicators                 []string
	WebsocketURL                string
	BehindHTTPSProxy            bool
	SegmentDebugDir             string
	AdminDIDs                   []string
	Syndicate                   []string
	PlayerTelemetry             bool
	PlaybackWorkerURL           string
	Ingests                     *placestream.IngestGetIngestUrls_Output
	S3Endpoint                  string
	S3Bucket                    string
	S3AccessKeyID               string
	S3SecretAccessKey           string
	S3Region                    string
	VODCDNURL                   string
	DisableSyndication          bool
	MuxlInitialMemoryMB         int
	MuxlMaxMemoryMB             int
	GamesAPIURL                 string
	GamesAPIClientKey           string
	GamesAPIClientSecret        string
	BetaInviteDID               string
	ViewLogFlushInterval        time.Duration
	ViewCountAggregateInterval  time.Duration
	ViewCountAggregateLag       time.Duration
	VODConcurrency              int
	MaximumLiveBitrate          int
	SweepConcurrency            int
	SweepInterval               time.Duration
	SweepBootDelay              time.Duration
	DeepenRate                  int
	FirehoseReplayWindow        time.Duration
	IndexDBConnections          int
}

// DefaultSweepInterval is how often the atproto sweep re-runs when
// --sweep-interval is unset.
//
// The sweep's first pass over a repo that is up to date is a single
// getLatestCommit, so this is a per-repo request budget: six hours means an
// indexed account is asked about four times a day, and drift -- a gap in the
// firehose, a span missed while this node was down -- is found and repaired
// within that. Any lower buys hours of detection latency for a proportional
// increase in traffic against every PDS on the network.
const DefaultSweepInterval = 6 * time.Hour

// DefaultSweepConcurrency is how many PDS hosts the atproto backfill sweep
// works on at once when --sweep-concurrency is unset or zero.
//
// The sweep shards its work by host and gives each host one worker, so this
// bounds remote servers rather than repos: 32 of them is a few hundred requests
// per second spread across the whole network, and no more than one walk (5-7
// requests per second) against any single PDS.
const DefaultSweepConcurrency = 32

// DefaultDeepenRate is how many history windows a node walks per minute when
// --deepen-rate is unset.
//
// History acquisition is the one part of the sync engine nothing waits for: a
// repo's recent records are indexed by its shallow sync in seconds, and
// everything older is a background trickle. Running it flat out is what a node
// does exactly once -- at boot, where it replays years of every account's chat
// as fast as the network allows and buries the reconciliation the node actually
// serves from. So it is paced instead: 60 windows a minute is one window a
// second across the whole node, which a fresh 20k-repo index's full history
// (four to five windows a repo, 100k of them) trickles in over roughly a day.
// Deliberately: nothing is waiting for it. 0 removes the cap entirely, which is
// what an operator uses to rush an initial build in place; negative means this
// default.
const DefaultDeepenRate = 60

// DefaultFirehoseReplayWindow is how stale a stored relay cursor may be before
// this node stops trying to replay from it and tails the live edge instead.
//
// The firehose is a latency optimization, not the sync engine: the sweep's head
// check asks every repo's host one question and repairs the ones that have
// drifted, so a gap of hours costs a few thousand cheap requests spread across
// hundreds of hosts. Replaying that same gap costs the relay a full-rate flood
// of every commit on the network -- including the overwhelming majority from
// repos this node has never heard of -- which is how a two-hour-old cursor once
// buried a node under millions of queued events. Fifteen minutes is long enough
// to cover an ordinary restart or deploy, where replay genuinely is the cheaper
// answer, and short enough that anything worse is handed to the mechanism built
// for it. 0 disables the cap and always replays from the stored cursor.
const DefaultFirehoseReplayWindow = 15 * time.Minute

// DefaultIndexDBConnections is how many sqlite connections the index database
// pool holds. See --index-db-connections; 1 is the fallback to the historical
// single-connection arrangement.
const DefaultIndexDBConnections = 8

// DefaultSweepBootDelay is how long a warm-index boot holds its first sweep
// (and the deepener's first scan). An ordinary upgrade-restart's gap is healed
// by the firehose replaying from the stored cursor, so the boot sweep is
// insurance, not repair -- it only needs to wait out the restart churn itself:
// streams reconnecting, the replay catching up, caches warming. Two minutes
// does that. Deliberately NOT longer: with deepening rate-capped and sweep
// passes reduced to checks and repairs, the pass is either harmless -- in
// which case it may as well run while the operator who just deployed is still
// watching the graphs -- or it is a problem, and a longer delay only schedules
// the problem for the moment they have stopped looking. A fresh index sweeps
// immediately regardless, and a cursor too stale to replay kicks its own
// sweep, so the cases that genuinely need boot-time sweeping keep it.
const DefaultSweepBootDelay = 2 * time.Minute

// ContentFilters represents the content filtering configuration
type ContentFilters struct {
	ContentWarnings struct {
		Enabled         bool     `json:"enabled"`
		BlockedWarnings []string `json:"blocked_warnings"`
	} `json:"content_warnings"`
	DistributionPolicy struct {
		Enabled bool `json:"enabled"`
	} `json:"distribution_policy"`
}

const (
	ReplicatorWebsocket string = "websocket"
	ReplicatorIroh      string = "iroh"
)

var LivepeerFlagSet *flag.FlagSet
var LivepeerConfig starter.LivepeerConfig

func (cli *CLI) NewCommand(name string) *urfavecli.Command {
	cmd := &urfavecli.Command{
		Name:  name,
		Usage: "streamplace server",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:        "data-dir",
				Usage:       "directory for keeping all streamplace data",
				Value:       DefaultDataDir(),
				Destination: &cli.DataDir,
				Sources:     urfavecli.EnvVars("SP_DATA_DIR"),
			},
			&urfavecli.StringFlag{
				Name:        "http-addr",
				Usage:       "Public HTTP address",
				Value:       ":38080",
				Destination: &cli.HTTPAddr,
				Sources:     urfavecli.EnvVars("SP_HTTP_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:        "http-internal-addr",
				Usage:       "Private, admin-only HTTP address",
				Value:       "127.0.0.1:39090",
				Destination: &cli.HTTPInternalAddr,
				Sources:     urfavecli.EnvVars("SP_HTTP_INTERNAL_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:        "https-addr",
				Usage:       "Public HTTPS address",
				Value:       ":38443",
				Destination: &cli.HTTPSAddr,
				Sources:     urfavecli.EnvVars("SP_HTTPS_ADDR"),
			},
			&urfavecli.BoolFlag{
				Name:        "secure",
				Usage:       "Run with HTTPS. Required for WebRTC output",
				Value:       false,
				Destination: &cli.Secure,
				Sources:     urfavecli.EnvVars("SP_SECURE"),
			},
			&urfavecli.BoolFlag{
				Name:        "isolated-ingest",
				Usage:       "Run each MKV/RTMP-push ingest in an isolated worker subprocess (fault isolation)",
				Value:       true,
				Destination: &cli.IsolatedIngest,
				Sources:     urfavecli.EnvVars("SP_ISOLATED_INGEST"),
			},
			&urfavecli.BoolFlag{
				Name:        "ll-hls",
				Usage:       "enable the experimental low-latency HLS origin for RTMP H.264/AAC and WHIP H.264/Opus streams",
				Value:       false,
				Destination: &cli.LLHLS,
				Sources:     urfavecli.EnvVars("SP_LL_HLS"),
			},
			&urfavecli.StringFlag{
				Name:        "tls-cert",
				Usage:       fmt.Sprintf(`Path to TLS certificate (default: "%s")`, filepath.Join(SPDataDir, "tls", "tls.crt")),
				Destination: &cli.TLSCertPath,
				Value:       filepath.Join(SPDataDir, "tls", "tls.crt"),
				Sources:     urfavecli.EnvVars("SP_TLS_CERT"),
			},
			&urfavecli.StringFlag{
				Name:        "tls-key",
				Usage:       fmt.Sprintf(`Path to TLS key (default: "%s")`, filepath.Join(SPDataDir, "tls", "tls.key")),
				Destination: &cli.TLSKeyPath,
				Value:       filepath.Join(SPDataDir, "tls", "tls.key"),
				Sources:     urfavecli.EnvVars("SP_TLS_KEY"),
			},
			&urfavecli.StringFlag{
				Name:        "signing-key",
				Usage:       "Path to signing key for pushing OTA updates to the app",
				Destination: &cli.SigningKeyPath,
				Sources:     urfavecli.EnvVars("SP_SIGNING_KEY"),
			},
			&urfavecli.StringFlag{
				Name:        "db-url",
				Usage:       "URL of the database to use for storing private streamplace state",
				Value:       "sqlite://$SP_DATA_DIR/state.sqlite",
				Destination: &cli.DBURL,
				Sources:     urfavecli.EnvVars("SP_DB_URL"),
			},
			&urfavecli.StringFlag{
				Name:        "admin-account",
				Usage:       "ethereum account that administrates this streamplace node",
				Destination: &cli.AdminAccount,
				Sources:     urfavecli.EnvVars("SP_ADMIN_ACCOUNT"),
			},
			&urfavecli.StringFlag{
				Name:        "firebase-service-account",
				Usage:       "Base64-encoded JSON string of a firebase service account key",
				Destination: &cli.FirebaseServiceAccount,
				Sources:     urfavecli.EnvVars("SP_FIREBASE_SERVICE_ACCOUNT"),
			},
			&urfavecli.StringFlag{
				Name:        "firebase-service-account-file",
				Usage:       "Path to a JSON file containing a firebase service account key",
				Destination: &cli.FirebaseServiceAccountFile,
				Sources:     urfavecli.EnvVars("SP_FIREBASE_SERVICE_ACCOUNT_FILE"),
			},
			&urfavecli.StringFlag{
				Name:        "gitlab-url",
				Usage:       "gitlab url for generating download links",
				Value:       "https://git.stream.place/api/v4/projects/1",
				Destination: &cli.GitLabURL,
				Sources:     urfavecli.EnvVars("SP_GITLAB_URL"),
			},
			&urfavecli.StringFlag{
				Name:        "eth-keystore-path",
				Usage:       fmt.Sprintf(`path to ethereum keystore (default: "%s")`, filepath.Join(SPDataDir, "keystore")),
				Destination: &cli.EthKeystorePath,
				Value:       filepath.Join(SPDataDir, "keystore"),
				Sources:     urfavecli.EnvVars("SP_ETH_KEYSTORE_PATH"),
			},
			&urfavecli.StringFlag{
				Name:        "eth-account-addr",
				Usage:       "ethereum account address to use (if keystore contains more than one)",
				Destination: &cli.EthAccountAddr,
				Sources:     urfavecli.EnvVars("SP_ETH_ACCOUNT_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:        "eth-password",
				Usage:       "password for encrypting keystore",
				Destination: &cli.EthPassword,
				Sources:     urfavecli.EnvVars("SP_ETH_PASSWORD"),
			},
			&urfavecli.StringFlag{
				Name:        "ta-url",
				Usage:       "timestamp authority server for signing",
				Value:       "http://timestamp.digicert.com",
				Destination: &cli.TAURL,
				Sources:     urfavecli.EnvVars("SP_TA_URL"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-module-path",
				Usage:       "path to a PKCS11 module for HSM signing, for example /usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
				Destination: &cli.PKCS11ModulePath,
				Sources:     urfavecli.EnvVars("SP_PKCS11_MODULE_PATH"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-pin",
				Usage:       "PIN for logging into PKCS11 token. if not provided, will be prompted interactively",
				Destination: &cli.PKCS11Pin,
				Sources:     urfavecli.EnvVars("SP_PKCS11_PIN"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-token-slot",
				Usage:       "slot number of PKCS11 token (only use one of slot, label, or serial)",
				Destination: &cli.PKCS11TokenSlot,
				Sources:     urfavecli.EnvVars("SP_PKCS11_TOKEN_SLOT"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-token-label",
				Usage:       "label of PKCS11 token (only use one of slot, label, or serial)",
				Destination: &cli.PKCS11TokenLabel,
				Sources:     urfavecli.EnvVars("SP_PKCS11_TOKEN_LABEL"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-token-serial",
				Usage:       "serial number of PKCS11 token (only use one of slot, label, or serial)",
				Destination: &cli.PKCS11TokenSerial,
				Sources:     urfavecli.EnvVars("SP_PKCS11_TOKEN_SERIAL"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-keypair-label",
				Usage:       "label of signing keypair on PKCS11 token",
				Destination: &cli.PKCS11KeypairLabel,
				Sources:     urfavecli.EnvVars("SP_PKCS11_KEYPAIR_LABEL"),
			},
			&urfavecli.StringFlag{
				Name:        "pkcs11-keypair-id",
				Usage:       "id of signing keypair on PKCS11 token",
				Destination: &cli.PKCS11KeypairID,
				Sources:     urfavecli.EnvVars("SP_PKCS11_KEYPAIR_ID"),
			},
			&urfavecli.StringFlag{
				Name:        "app-bundle-id",
				Usage:       "bundle id of an app that we facilitate oauth login for",
				Destination: &cli.AppBundleID,
				Sources:     urfavecli.EnvVars("SP_APP_BUNDLE_ID"),
			},
			&urfavecli.StringFlag{
				Name:        "streamer-name",
				Usage:       "name of the person streaming from this streamplace node",
				Destination: &cli.StreamerName,
				Sources:     urfavecli.EnvVars("SP_STREAMER_NAME"),
			},
			&urfavecli.StringFlag{
				Name:        "dev-frontend-proxy",
				Usage:       "(FOR DEVELOPMENT ONLY) proxy frontend requests to this address instead of using the bundled frontend",
				Destination: &cli.FrontendProxy,
				Sources:     urfavecli.EnvVars("SP_DEV_FRONTEND_PROXY"),
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "false" {
						cli.FrontendProxy = ""
						return nil
					}
					cli.FrontendProxy = s
					return nil
				},
			},
			&urfavecli.StringFlag{
				Name:        "frontend",
				Usage:       "which bundled frontend to serve: 'app' (legacy Expo) or 'web' (Vite)",
				Value:       "app",
				Destination: &cli.Frontend,
				Sources:     urfavecli.EnvVars("SP_FRONTEND"),
			},
			&urfavecli.BoolFlag{
				Name:        "dev-public-oauth",
				Usage:       "(FOR DEVELOPMENT ONLY) enable public oauth login for http://127.0.0.1 development",
				Value:       false,
				Destination: &cli.PublicOAuth,
				Sources:     urfavecli.EnvVars("SP_DEV_PUBLIC_OAUTH"),
			},
			&urfavecli.StringFlag{
				Name:        "livepeer-gateway-url",
				Usage:       "URL of the Livepeer Gateway to use for transcoding",
				Destination: &cli.LivepeerGatewayURL,
				Sources:     urfavecli.EnvVars("SP_LIVEPEER_GATEWAY_URL"),
			},
			&urfavecli.BoolFlag{
				Name:        "livepeer-gateway",
				Usage:       "enable embedded Livepeer Gateway",
				Value:       false,
				Destination: &cli.LivepeerGateway,
				Sources:     urfavecli.EnvVars("SP_LIVEPEER_GATEWAY"),
			},
			&urfavecli.BoolFlag{
				Name:        "wide-open",
				Usage:       "allow ALL streams to be uploaded to this node (not recommended for production)",
				Value:       false,
				Destination: &cli.WideOpen,
				Sources:     urfavecli.EnvVars("SP_WIDE_OPEN"),
			},
			&urfavecli.StringFlag{
				Name:  "allowed-streams",
				Usage: `if set, only allow these addresses or atproto DIDs to upload to this node (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.AllowedStreams = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_ALLOWED_STREAMS"),
			},
			&urfavecli.StringFlag{
				Name:  "peers",
				Usage: `other streamplace nodes to replicate to (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.Peers = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_PEERS"),
			},
			&urfavecli.StringFlag{
				Name:  "redirects",
				Usage: `http 302s /path/one:/path/two,/path/three:/path/four (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					return json.Unmarshal([]byte(s), &cli.Redirects)
				},
				Sources: urfavecli.EnvVars("SP_REDIRECTS"),
			},
			&urfavecli.StringFlag{
				Name:  "debug",
				Usage: "modified log verbosity for specific functions or files in form func=ToHLS:3,file=gstreamer.go:4",
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.Debug = map[string]map[string]int{}
					pairs := strings.SplitSeq(s, ",")
					for pair := range pairs {
						scoreSplit := strings.Split(pair, ":")
						if len(scoreSplit) != 2 {
							return fmt.Errorf("invalid debug flag: %s", pair)
						}
						score, err := strconv.Atoi(scoreSplit[1])
						if err != nil {
							return fmt.Errorf("invalid debug flag: %s", pair)
						}
						selectorSplit := strings.Split(scoreSplit[0], "=")
						if len(selectorSplit) != 2 {
							return fmt.Errorf("invalid debug flag: %s", pair)
						}
						_, ok := cli.Debug[selectorSplit[0]]
						if !ok {
							cli.Debug[selectorSplit[0]] = map[string]int{}
						}
						cli.Debug[selectorSplit[0]][selectorSplit[1]] = score
					}
					return nil
				},
				Sources: urfavecli.EnvVars("SP_DEBUG"),
			},
			&urfavecli.BoolFlag{
				Name:        "test-stream",
				Usage:       "run a built-in test stream on boot",
				Value:       false,
				Destination: &cli.TestStream,
				Sources:     urfavecli.EnvVars("SP_TEST_STREAM"),
			},
			&urfavecli.BoolFlag{
				Name:        "no-firehose",
				Usage:       "disable the bluesky firehose",
				Value:       false,
				Destination: &cli.NoFirehose,
				Sources:     urfavecli.EnvVars("SP_NO_FIREHOSE"),
			},
			&urfavecli.BoolFlag{
				Name:        "print-chat",
				Usage:       "print chat messages to stdout",
				Value:       false,
				Destination: &cli.PrintChat,
				Sources:     urfavecli.EnvVars("SP_PRINT_CHAT"),
			},
			&urfavecli.StringFlag{
				Name:        "whip-test",
				Usage:       "run a WHIP self-test with the given parameters",
				Destination: &cli.WHIPTest,
				Sources:     urfavecli.EnvVars("SP_WHIP_TEST"),
			},
			&urfavecli.StringFlag{
				Name:        "relay-host",
				Usage:       "comma-separated url(s) for relay firehose(s); ws://, wss://, or moqt:// (MoQ-over-QUIC) relays may be mixed. Subscribing to several relays survives any one going down (duplicate events are deduped). Our own PDS firehose is always included so locally-published records are indexed immediately",
				Value:       "wss://bsky.network",
				Destination: &cli.RelayHost,
				Sources:     urfavecli.EnvVars("SP_RELAY_HOST"),
			},
			&urfavecli.StringFlag{
				Name:        "color",
				Usage:       "'true' to enable colorized logging, 'false' to disable",
				Destination: &cli.Color,
				Sources:     urfavecli.EnvVars("SP_COLOR"),
			},
			&urfavecli.StringFlag{
				Name:        "broadcaster-host",
				Usage:       "public host for the broadcaster group that this node is a part of (excluding https:// e.g. stream.place)",
				Destination: &cli.BroadcasterHost,
				Sources:     urfavecli.EnvVars("SP_BROADCASTER_HOST"),
			},
			&urfavecli.StringFlag{
				Name:        "public-host",
				Usage:       "deprecated, use broadcaster-host or server-host instead as appropriate",
				Destination: &cli.XXDeprecatedPublicHost,
				Sources:     urfavecli.EnvVars("SP_PUBLIC_HOST"),
			},
			&urfavecli.StringFlag{
				Name:        "server-host",
				Usage:       "public host for this particular physical streamplace node. defaults to broadcaster-host and only must be set for multi-node broadcasters",
				Destination: &cli.ServerHost,
				Sources:     urfavecli.EnvVars("SP_SERVER_HOST"),
			},
			&urfavecli.BoolFlag{
				Name:        "thumbnail",
				Usage:       "enable thumbnail generation",
				Value:       true,
				Destination: &cli.Thumbnail,
				Sources:     urfavecli.EnvVars("SP_THUMBNAIL"),
			},
			&urfavecli.StringFlag{
				Name:        "tracing-endpoint",
				Usage:       "gRPC endpoint to send traces to",
				Destination: &cli.TracingEndpoint,
				Sources:     urfavecli.EnvVars("SP_TRACING_ENDPOINT"),
			},
			&urfavecli.IntFlag{
				Name:        "rate-limit-per-second",
				Usage:       "rate limit for requests per second per ip",
				Value:       0,
				Destination: &cli.RateLimitPerSecond,
				Sources:     urfavecli.EnvVars("SP_RATE_LIMIT_PER_SECOND"),
			},
			&urfavecli.IntFlag{
				Name:        "rate-limit-burst",
				Usage:       "rate limit burst for requests per ip",
				Value:       0,
				Destination: &cli.RateLimitBurst,
				Sources:     urfavecli.EnvVars("SP_RATE_LIMIT_BURST"),
			},
			&urfavecli.IntFlag{
				Name:        "rate-limit-websocket",
				Usage:       "number of concurrent websocket connections allowed per ip",
				Value:       10,
				Destination: &cli.RateLimitWebsocket,
				Sources:     urfavecli.EnvVars("SP_RATE_LIMIT_WEBSOCKET"),
			},
			&urfavecli.IntFlag{
				Name:        "muxl-initial-memory-mb",
				Usage:       "initial wasm linear memory pre-allocation per muxl instance, in MiB. higher avoids realloc churn at the cost of holding more memory upfront",
				Value:       50,
				Destination: &cli.MuxlInitialMemoryMB,
				Sources:     urfavecli.EnvVars("SP_MUXL_INITIAL_MEMORY_MB"),
			},
			&urfavecli.IntFlag{
				Name:        "muxl-max-memory-mb",
				Usage:       "hard ceiling on wasm linear memory per muxl instance, in MiB. signing fails if a segment requires more than this",
				Value:       1024,
				Destination: &cli.MuxlMaxMemoryMB,
				Sources:     urfavecli.EnvVars("SP_MUXL_MAX_MEMORY_MB"),
			},
			&urfavecli.StringFlag{
				Name:        "rtmp-server-addon",
				Usage:       "address of external RTMP server to forward streams to",
				Destination: &cli.RTMPServerAddon,
				Sources:     urfavecli.EnvVars("SP_RTMP_SERVER_ADDON"),
			},
			&urfavecli.StringFlag{
				Name:        "rtmps-addon-addr",
				Usage:       "address to listen for RTMPS on the addon server",
				Value:       ":1936",
				Destination: &cli.RTMPSAddonAddr,
				Sources:     urfavecli.EnvVars("SP_RTMPS_ADDON_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:        "rtmps-addr",
				Usage:       "address to listen for RTMPS connections (when --secure=true)",
				Value:       ":1935",
				Destination: &cli.RTMPSAddr,
				Sources:     urfavecli.EnvVars("SP_RTMPS_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:        "rtmp-addr",
				Usage:       "address to listen for RTMP connections (when --secure=false)",
				Value:       ":1935",
				Destination: &cli.RTMPAddr,
				Sources:     urfavecli.EnvVars("SP_RTMP_ADDR"),
			},
			&urfavecli.StringFlag{
				Name:  "discord-webhooks",
				Usage: `JSON array of Discord webhooks to send notifications to (default: "[]")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					return json.Unmarshal([]byte(s), &cli.DiscordWebhooks)
				},
				Sources: urfavecli.EnvVars("SP_DISCORD_WEBHOOKS"),
			},
			&urfavecli.StringFlag{
				Name:        "apple-team-id",
				Usage:       "apple team id for deep linking",
				Destination: &cli.AppleTeamID,
				Sources:     urfavecli.EnvVars("SP_APPLE_TEAM_ID"),
			},
			&urfavecli.StringFlag{
				Name:        "android-cert-fingerprint",
				Usage:       "android cert fingerprint for deep linking",
				Destination: &cli.AndroidCertFingerprint,
				Sources:     urfavecli.EnvVars("SP_ANDROID_CERT_FINGERPRINT"),
			},
			&urfavecli.StringFlag{
				Name:  "labelers",
				Usage: `did of labelers that this instance should subscribe to (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.Labelers = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_LABELERS"),
			},
			&urfavecli.StringFlag{
				Name:        "atproto-did",
				Usage:       "atproto did to respond to on /.well-known/atproto-did (default did:web:PUBLIC_HOST)",
				Destination: &cli.AtprotoDID,
				Sources:     urfavecli.EnvVars("SP_ATPROTO_DID"),
			},
			&urfavecli.StringFlag{
				Name:  "content-filters",
				Usage: `JSON content filtering rules (default: "{}")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					return json.Unmarshal([]byte(s), &cli.ContentFilters)
				},
				Sources: urfavecli.EnvVars("SP_CONTENT_FILTERS"),
			},
			&urfavecli.StringFlag{
				Name:        "moderation-dir",
				Usage:       "directory containing additional .txt profanity wordlists to load at startup",
				Destination: &cli.ModerationDir,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					moderation.ModerationDir = s
					return nil
				},
				Sources: urfavecli.EnvVars("SP_MODERATION_DIR"),
			},
			&urfavecli.StringFlag{
				Name:  "default-recommended-streamers",
				Usage: `comma-separated list of streamer DIDs to recommend by default when no other recommendations are available (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.DefaultRecommendedStreamers = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_DEFAULT_RECOMMENDED_STREAMERS"),
			},
			&urfavecli.BoolFlag{
				Name:        "livepeer-help",
				Usage:       "print help for livepeer flags and exit",
				Value:       false,
				Destination: &cli.LivepeerHelp,
				Sources:     urfavecli.EnvVars("SP_LIVEPEER_HELP"),
			},
			&urfavecli.StringFlag{
				Name:        "plc-url",
				Usage:       "url of the plc directory",
				Value:       "https://plc.directory",
				Destination: &cli.PLCURL,
				Sources:     urfavecli.EnvVars("SP_PLC_URL"),
			},
			&urfavecli.BoolFlag{
				Name:        "sql-logging",
				Usage:       "enable sql logging",
				Value:       false,
				Destination: &cli.SQLLogging,
				Sources:     urfavecli.EnvVars("SP_SQL_LOGGING"),
			},
			&urfavecli.StringFlag{
				Name:        "sentry-dsn",
				Usage:       "sentry dsn for error reporting",
				Destination: &cli.SentryDSN,
				Sources:     urfavecli.EnvVars("SP_SENTRY_DSN"),
			},
			&urfavecli.StringFlag{
				Name:        "playback-worker-url",
				Usage:       "URL of the Cloudflare playback router worker",
				Destination: &cli.PlaybackWorkerURL,
				Sources:     urfavecli.EnvVars("SP_PLAYBACK_WORKER_URL"),
			},
			&urfavecli.StringFlag{
				Name:        "games-api-url",
				Usage:       "URL of the games.gamesgamesgamesgames API (e.g. http://localhost:3001)",
				Destination: &cli.GamesAPIURL,
				Sources:     urfavecli.EnvVars("SP_GAMES_API_URL"),
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					cli.GamesAPIURL = strings.TrimRight(s, "/")
					return nil
				},
			},
			&urfavecli.StringFlag{
				Name:        "games-api-client-key",
				Usage:       "Client key for authenticating with the games.gamesgamesgamesgames API",
				Destination: &cli.GamesAPIClientKey,
				Sources:     urfavecli.EnvVars("SP_GAMES_API_CLIENT_KEY"),
			},
			&urfavecli.StringFlag{
				Name:        "games-api-client-secret",
				Usage:       "Client secret for authenticating with the games.gamesgamesgamesgames API",
				Destination: &cli.GamesAPIClientSecret,
				Sources:     urfavecli.EnvVars("SP_GAMES_API_CLIENT_SECRET"),
			},
			&urfavecli.BoolFlag{
				Name:        "livepeer-debug",
				Usage:       "log livepeer segments to $SP_DATA_DIR/livepeer-debug",
				Value:       false,
				Destination: &cli.LivepeerDebug,
				Sources:     urfavecli.EnvVars("SP_LIVEPEER_DEBUG"),
			},
			&urfavecli.StringFlag{
				Name:        "segment-debug-dir",
				Usage:       "directory to log segment validation to",
				Destination: &cli.SegmentDebugDir,
				Sources:     urfavecli.EnvVars("SP_SEGMENT_DEBUG_DIR"),
			},
			&urfavecli.StringFlag{
				Name:  "tickets",
				Usage: `tickets to join the swarm with (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.Tickets = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_TICKETS"),
			},
			&urfavecli.StringFlag{
				Name:        "iroh-topic",
				Usage:       "topic to use for the iroh swarm (must be 32 bytes in hex)",
				Destination: &cli.IrohTopic,
				Sources:     urfavecli.EnvVars("SP_IROH_TOPIC"),
			},
			&urfavecli.BoolFlag{
				Name:        "disable-iroh-relay",
				Usage:       "disable the iroh relay",
				Value:       false,
				Destination: &cli.DisableIrohRelay,
				Sources:     urfavecli.EnvVars("SP_DISABLE_IROH_RELAY"),
			},
			&urfavecli.StringFlag{
				Name:  "dev-account-creds",
				Usage: `(FOR DEVELOPMENT ONLY) did=password pairs for logging into test accounts without oauth (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.DevAccountCreds = map[string]string{}
					pairs := strings.Split(s, ",")
					for _, pair := range pairs {
						parts := strings.Split(pair, "=")
						if len(parts) != 2 {
							return fmt.Errorf("invalid kv flag: %s", pair)
						}
						cli.DevAccountCreds[parts[0]] = parts[1]
					}
					return nil
				},
				Sources: urfavecli.EnvVars("SP_DEV_ACCOUNT_CREDS"),
			},
			&urfavecli.DurationFlag{
				Name:        "stream-session-timeout",
				Usage:       "how long to wait before considering a stream inactive on this node?",
				Value:       60 * time.Second,
				Destination: &cli.StreamSessionTimeout,
				Sources:     urfavecli.EnvVars("SP_STREAM_SESSION_TIMEOUT"),
			},
			&urfavecli.IntFlag{
				Name:        "vod-concurrency",
				Usage:       "number of VOD processing tasks to run in parallel on this node",
				Value:       2,
				Destination: &cli.VODConcurrency,
				Sources:     urfavecli.EnvVars("SP_VOD_CONCURRENCY"),
			},
			&urfavecli.IntFlag{
				Name:        "sweep-concurrency",
				Usage:       "how many PDS hosts the atproto backfill sweep talks to at once. Work is sharded by host and each host is walked by one worker, so this is a count of remote servers, not of repos; 0 for the default",
				Value:       DefaultSweepConcurrency,
				Destination: &cli.SweepConcurrency,
				Sources:     urfavecli.EnvVars("SP_SWEEP_CONCURRENCY"),
			},
			&urfavecli.IntFlag{
				Name:        "deepen-rate",
				Usage:       "how many history windows per minute this node walks in the background. History deepening is decoupled from the sweep -- the sweep checks and repairs, this fetches the past -- and nothing on the node waits for it, so it is paced rather than run flat out; 0 removes the cap (an initial build in a hurry), negative for the default",
				Value:       DefaultDeepenRate,
				Destination: &cli.DeepenRate,
				Sources:     urfavecli.EnvVars("SP_DEEPEN_RATE"),
			},
			&urfavecli.IntFlag{
				Name:        "index-db-connections",
				Usage:       "how many sqlite connections the index database pool holds. More than one lets reads run beside a reindex under WAL; 1 restores the old slower-but-safer single-connection arrangement; 0 for the default",
				Value:       DefaultIndexDBConnections,
				Destination: &cli.IndexDBConnections,
				Sources:     urfavecli.EnvVars("SP_INDEX_DB_CONNECTIONS"),
			},
			&urfavecli.DurationFlag{
				Name:        "sweep-boot-delay",
				Usage:       "how long a node with a warm index waits after boot before its first sweep, so the sweep's reindexing does not compound the busiest minutes of a restart. A fresh (empty) index always sweeps immediately, as does a --no-firehose node (no replay heals its gap), and 0 sweeps immediately in every case",
				Value:       DefaultSweepBootDelay,
				Destination: &cli.SweepBootDelay,
				Sources:     urfavecli.EnvVars("SP_SWEEP_BOOT_DELAY"),
			},
			&urfavecli.DurationFlag{
				Name:        "sweep-interval",
				Usage:       "how often to re-run the atproto sweep, which asks every indexed repo's host whether our copy is still current and repairs the ones that are not. 0 disables re-running; the sweep at startup always happens",
				Value:       DefaultSweepInterval,
				Destination: &cli.SweepInterval,
				Sources:     urfavecli.EnvVars("SP_SWEEP_INTERVAL"),
			},
			&urfavecli.DurationFlag{
				Name:        "firehose-replay-window",
				Usage:       "how old a stored relay cursor may be and still be replayed from on connect. A cursor whose newest event is older than this is discarded and we tail the relay's live edge instead, leaving the gap for the sweep's head check to repair -- which is far cheaper than making the relay re-send every commit on the network. 0 always replays from the stored cursor",
				Value:       DefaultFirehoseReplayWindow,
				Destination: &cli.FirehoseReplayWindow,
				Sources:     urfavecli.EnvVars("SP_FIREHOSE_REPLAY_WINDOW"),
			},
			&urfavecli.StringFlag{
				Name:    "maximum-live-bitrate",
				Usage:   "maximum allowed live ingest bitrate, measured per emitted segment. Accepts a bits-per-second number or a decimal SI suffix — e.g. 30M, 30000k, or 30000000 (all 30 Mbps). A stream whose bitrate exceeds this (plus a 10% margin) is disconnected and the streamer is shown a problem. 0 = unlimited",
				Value:   "0",
				Sources: urfavecli.EnvVars("SP_MAXIMUM_LIVE_BITRATE"),
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					v, err := ParseSI(s)
					if err != nil {
						return fmt.Errorf("invalid --maximum-live-bitrate: %w", err)
					}
					cli.MaximumLiveBitrate = int(v)
					return nil
				},
			},
			&urfavecli.BoolFlag{
				Name:        "legacy-segment-cleaner",
				Usage:       "re-enable the legacy segment cleaner. shouldn't be needed but can be useful in cases where localdb is too big.",
				Value:       false,
				Destination: &cli.LegacySegmentCleaner,
				Sources:     urfavecli.EnvVars("SP_LEGACY_SEGMENT_CLEANER"),
			},
			&urfavecli.DurationFlag{
				Name:        "segment-archive-retention",
				Usage:       "how long to keep on-disk segment files before cleaning them up (durable copies live in S3/VOD). 0 disables cleanup.",
				Value:       1 * time.Hour,
				Destination: &cli.SegmentArchiveRetention,
				Sources:     urfavecli.EnvVars("SP_SEGMENT_ARCHIVE_RETENTION"),
			},
			&urfavecli.StringFlag{
				Name:    "replicators",
				Usage:   "comma-separated list of replication protocols to use (websocket, iroh)",
				Value:   ReplicatorWebsocket,
				Sources: urfavecli.EnvVars("SP_REPLICATORS"),
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s != "" {
						cli.Replicators = strings.Split(s, ",")
					}
					return nil
				},
			},
			&urfavecli.StringFlag{
				Name:        "websocket-url",
				Usage:       "override the websocket (ws:// or wss://) url to use for replication (normally not necessary, used for testing)",
				Destination: &cli.WebsocketURL,
				Sources:     urfavecli.EnvVars("SP_WEBSOCKET_URL"),
			},
			&urfavecli.BoolFlag{
				Name:        "behind-https-proxy",
				Usage:       "set to true if this node is behind an https proxy and we should report https URLs even though the node isn't serving HTTPS",
				Value:       false,
				Destination: &cli.BehindHTTPSProxy,
				Sources:     urfavecli.EnvVars("SP_BEHIND_HTTPS_PROXY"),
			},
			&urfavecli.StringFlag{
				Name:  "admin-dids",
				Usage: `comma-separated list of DIDs that are authorized to modify branding and other admin operations (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.AdminDIDs = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_ADMIN_DIDS"),
			},
			&urfavecli.StringFlag{
				Name:  "syndicate",
				Usage: `list of DIDs that we should rebroadcast ('*' for everybody) (default: "")`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					cli.Syndicate = strings.Split(s, ",")
					return nil
				},
				Sources: urfavecli.EnvVars("SP_SYNDICATE"),
			},
			&urfavecli.BoolFlag{
				Name:        "disable-syndication",
				Usage:       `entirely disable syndication in both directions. useful for local development.`,
				Value:       false,
				Destination: &cli.DisableSyndication,
				Sources:     urfavecli.EnvVars("SP_DISABLE_SYNDICATION"),
			},
			&urfavecli.BoolFlag{
				Name:        "player-telemetry",
				Usage:       "enable player telemetry",
				Value:       true,
				Destination: &cli.PlayerTelemetry,
				Sources:     urfavecli.EnvVars("SP_PLAYER_TELEMETRY"),
			},
			&urfavecli.StringFlag{
				Name:        "local-db-url",
				Usage:       "URL of the local database to use for storing local data",
				Value:       "sqlite://$SP_DATA_DIR/localdb.sqlite",
				Destination: &cli.LocalDBURL,
				Sources:     urfavecli.EnvVars("SP_LOCAL_DB_URL"),
			},
			&urfavecli.StringFlag{
				Name:  "ingests",
				Usage: `JSON array of ingests to return from place.stream.ingest.getIngestUrls. Default is auto-generated ingests for RTMP and WHIP`,
				Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
					if s == "" {
						return nil
					}
					return json.Unmarshal([]byte(s), &cli.Ingests)
				},
				Sources: urfavecli.EnvVars("SP_INGESTS"),
			},
			&urfavecli.StringFlag{
				Name:        "s3-endpoint",
				Usage:       "S3-compatible endpoint URL for segment archival uploads",
				Destination: &cli.S3Endpoint,
				Sources:     urfavecli.EnvVars("SP_S3_ENDPOINT"),
			},
			&urfavecli.StringFlag{
				Name:        "s3-bucket",
				Usage:       "S3 bucket name for segment archival uploads",
				Destination: &cli.S3Bucket,
				Sources:     urfavecli.EnvVars("SP_S3_BUCKET"),
			},
			&urfavecli.StringFlag{
				Name:        "s3-access-key-id",
				Usage:       "S3 access key ID for segment archival uploads",
				Destination: &cli.S3AccessKeyID,
				Sources:     urfavecli.EnvVars("SP_S3_ACCESS_KEY_ID"),
			},
			&urfavecli.StringFlag{
				Name:        "s3-secret-access-key",
				Usage:       "S3 secret access key for segment archival uploads",
				Destination: &cli.S3SecretAccessKey,
				Sources:     urfavecli.EnvVars("SP_S3_SECRET_ACCESS_KEY"),
			},
			&urfavecli.StringFlag{
				Name:        "s3-region",
				Usage:       "S3 region (default: us-east-1)",
				Value:       "us-east-1",
				Destination: &cli.S3Region,
				Sources:     urfavecli.EnvVars("SP_S3_REGION"),
			},
			&urfavecli.StringFlag{
				Name:        "vod-cdn-url",
				Usage:       "Static CDN URL fronting the VOD blob store. When set, HLS playlists emit segment + init-segment URLs of the form <vod-cdn-url>/<cid>.mp4?did=...&sid=... instead of the self-hosted getVideoBlob endpoint. Omit for self-contained deployments.",
				Destination: &cli.VODCDNURL,
				Sources:     urfavecli.EnvVars("SP_VOD_CDN_URL"),
			},
			&urfavecli.StringFlag{
				Name:        "beta-invite-did",
				Usage:       "DID of the atproto account whose place.stream.beta.invite records this node trusts. When set, uploading VODs requires an invite from that account; when empty, falls back to the --allowed-streams allowlist used by livestreaming.",
				Destination: &cli.BetaInviteDID,
				Sources:     urfavecli.EnvVars("SP_BETA_INVITE_DID"),
			},
			&urfavecli.DurationFlag{
				Name:        "view-log-flush-interval",
				Usage:       "How often the view-log writer rotates its buffer to the VOD blob store. Set to 0 to disable view-event logging entirely (no view counts will be available downstream). Files land at view-logs/<server-did>/<window>.jsonl.gz alongside the VOD content blobs.",
				Value:       5 * time.Minute,
				Destination: &cli.ViewLogFlushInterval,
				Sources:     urfavecli.EnvVars("SP_VIEW_LOG_FLUSH_INTERVAL"),
			},
			&urfavecli.DurationFlag{
				Name:        "view-count-aggregate-interval",
				Usage:       "How often a node tries to enqueue a view-count aggregation task. Buckets align on UTC multiples of this interval; deduplication via statedb's unique task-key constraint ensures only one node per bucket actually runs the aggregation. Set to 0 to disable aggregation (capture continues but no place.stream.media.viewCount records are published).",
				Value:       5 * time.Minute,
				Destination: &cli.ViewCountAggregateInterval,
				Sources:     urfavecli.EnvVars("SP_VIEW_COUNT_AGGREGATE_INTERVAL"),
			},
			&urfavecli.DurationFlag{
				Name:        "view-count-aggregate-lag",
				Usage:       "How long the aggregator waits after a bucket closes before processing it, so all writers have time to flush their buffers. Should be at least one --view-log-flush-interval; default 2× that.",
				Value:       10 * time.Minute,
				Destination: &cli.ViewCountAggregateLag,
				Sources:     urfavecli.EnvVars("SP_VIEW_COUNT_AGGREGATE_LAG"),
			},
			&urfavecli.BoolFlag{
				Name:  "external-signing",
				Usage: "DEPRECATED, does nothing.",
				Value: true,
			},
			&urfavecli.BoolFlag{
				Name:  "insecure",
				Usage: "DEPRECATED, does nothing.",
				Value: false,
			},
		},
		Before: func(ctx context.Context, cmd *urfavecli.Command) (context.Context, error) {
			return ctx, cli.Validate(cmd)
		},
	}

	// Add data dir flags
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.DBURL)
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.LocalDBURL)
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.TLSCertPath)
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.TLSKeyPath)
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.EthKeystorePath)

	if runtime.GOOS == "linux" {
		cmd.Flags = append(cmd.Flags, &urfavecli.BoolFlag{
			Name:        "no-mist",
			Usage:       "Disable MistServer",
			Value:       true,
			Destination: &cli.NoMist,
			Sources:     urfavecli.EnvVars("SP_NO_MIST"),
		})
		cmd.Flags = append(cmd.Flags, &urfavecli.IntFlag{
			Name:        "mist-admin-port",
			Usage:       "MistServer admin port (internal use only)",
			Value:       14242,
			Destination: &cli.MistAdminPort,
			Sources:     urfavecli.EnvVars("SP_MIST_ADMIN_PORT"),
		})
		cmd.Flags = append(cmd.Flags, &urfavecli.IntFlag{
			Name:        "mist-rtmp-port",
			Usage:       "MistServer RTMP port (internal use only)",
			Value:       11935,
			Destination: &cli.MistRTMPPort,
			Sources:     urfavecli.EnvVars("SP_MIST_RTMP_PORT"),
		})
		cmd.Flags = append(cmd.Flags, &urfavecli.IntFlag{
			Name:        "mist-http-port",
			Usage:       "MistServer HTTP port (internal use only) — ingest pulls Mist's live fMP4 output from this port, so it must match the running Mist config (docker/mistserver.json uses 28080, the default here)",
			Value:       28080,
			Destination: &cli.MistHTTPPort,
			Sources:     urfavecli.EnvVars("SP_MIST_HTTP_PORT"),
		})

	}

	LivepeerFlagSet = flag.NewFlagSet("livepeer", flag.ContinueOnError)
	LivepeerConfig = starter.NewLivepeerConfig(LivepeerFlagSet)
	LivepeerFlagSet.VisitAll(func(f *flag.Flag) {
		adapted := LivepeerFlags.CamelToSnake[f.Name]
		cmd.Flags = append(cmd.Flags, &urfavecli.StringFlag{
			Name:    fmt.Sprintf("livepeer.%s", adapted),
			Usage:   f.Usage,
			Sources: urfavecli.EnvVars(fmt.Sprintf("SP_LIVEPEER_%s", adapted)),
			Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
				return LivepeerFlagSet.Set(f.Name, s)
			},
		})
	})

	return cmd
}

var StreamplaceSchemePrefix = "streamplace://"

// OwnPublicURL is the URL this process's own public listener answers on.
//
// With --secure we terminate TLS ourselves: the real handler is on HTTPSAddr
// and the HTTPAddr listener only serves 307 redirects to it (ServeHTTPRedirect),
// so http://<HTTPAddr> is not an address anything can actually be fetched from
// — a websocket dial there gets the redirect instead of a 101 upgrade.
// --behind-https-proxy is the opposite case: the proxy terminates TLS and we
// really do serve the handler as plain HTTP on HTTPAddr, so only cli.Secure
// flips this.
func (cli *CLI) OwnPublicURL() string {
	//  No errors because we know it's valid from AddrFlag
	addr, scheme := cli.HTTPAddr, "http"
	if cli.Secure {
		addr, scheme = cli.HTTPSAddr, "https"
	}
	host, port, _ := net.SplitHostPort(addr)

	ip := net.ParseIP(host)
	if host == "" || ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port))
}

func (cli *CLI) OwnInternalURL() string {
	//  No errors because we know it's valid from AddrFlag
	host, port, _ := net.SplitHostPort(cli.HTTPInternalAddr)

	ip := net.ParseIP(host)
	if ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, port)
	return fmt.Sprintf("http://%s", addr)
}

func (cli *CLI) ParseSigningKey() (*rsa.PrivateKey, error) {
	bs, err := os.ReadFile(cli.SigningKeyPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bs)
	if block == nil {
		return nil, fmt.Errorf("no RSA key found in signing key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func RandomTrailer(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	res := make([]byte, length)
	for i := 0; i < length; i++ {
		res[i] = charset[rand.IntN(len(charset))]
	}
	return string(res)
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// not fatal unless the user doesn't set one later
		return ""
	}
	return filepath.Join(home, ".streamplace")
}

var GormLogger = slogGorm.New(
	slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
		TimeFormat: time.RFC3339,
	})),
	slogGorm.WithTraceAll(),
)

func DisableSQLLogging() {
	GormLogger = slogGorm.New(
		slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.RFC3339,
		})),
	)
}

func EnableSQLLogging() {
	GormLogger = slogGorm.New(
		slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.RFC3339,
		})),
		slogGorm.WithTraceAll(),
	)
}

func (cli *CLI) Validate(cmd *urfavecli.Command) error {
	if cli.DataDir == "" {
		return fmt.Errorf("could not determine default data dir (no $HOME) and none provided, please set --data-dir")
	}
	if cli.LivepeerGateway && cli.LivepeerGatewayURL != "" {
		return fmt.Errorf("defining both livepeer-gateway and livepeer-gateway-url doesn't make sense. do you want an embedded gateway or an external one?")
	}
	if cli.LivepeerGateway {
		log.MonkeypatchStderr()
		// Livepeer gateway configuration will be handled in the caller
		cli.LivepeerGatewayURL = "http://127.0.0.1:8935"
	}
	for _, dest := range cli.dataDirFlags {
		*dest = strings.Replace(*dest, SPDataDir, cli.DataDir, 1)
	}
	if !cli.SQLLogging {
		DisableSQLLogging()
	} else {
		EnableSQLLogging()
	}
	if cli.XXDeprecatedPublicHost != "" && cli.BroadcasterHost == "" {
		log.Warn(context.Background(), "public-host is deprecated, use broadcaster-host or server-host instead as appropriate")
		cli.BroadcasterHost = cli.XXDeprecatedPublicHost
	}
	if cli.ServerHost == "" && cli.BroadcasterHost != "" {
		cli.ServerHost = cli.BroadcasterHost
	}
	if cli.PublicOAuth {
		log.Warn(context.Background(), "--dev-public-oauth is set, this is not recommended for production")
	}
	if cli.FirebaseServiceAccount != "" && cli.FirebaseServiceAccountFile != "" {
		return fmt.Errorf("defining both firebase-service-account and firebase-service-account-file doesn't make sense. do you want a base64-encoded string or a file?")
	}
	if cli.FirebaseServiceAccountFile != "" {
		bs, err := os.ReadFile(cli.FirebaseServiceAccountFile)
		if err != nil {
			return err
		}
		cli.FirebaseServiceAccount = string(bs)
	}
	// Set default replicator if none specified
	if len(cli.Replicators) == 0 {
		cli.Replicators = []string{ReplicatorWebsocket}
	}
	return nil
}

func (cli *CLI) DataFilePath(fpath []string) string {
	if cli.DataDir == "" {
		panic("no data dir configured")
	}
	// windows does not like colons
	safe := []string{}
	for _, p := range fpath {
		safe = append(safe, strings.ReplaceAll(p, ":", "-"))
	}
	fpath = append([]string{cli.DataDir}, safe...)
	fdpath := filepath.Join(fpath...)
	return fdpath
}

// does a file exist in our data dir?
func (cli *CLI) DataFileExists(fpath []string) (bool, error) {
	ddpath := cli.DataFilePath(fpath)
	_, err := os.Stat(ddpath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// write a file to our data dir
func (cli *CLI) DataFileWrite(fpath []string, r io.Reader, overwrite bool) error {
	fd, err := cli.DataFileCreate(fpath, overwrite)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = io.Copy(fd, r)
	if err != nil {
		return err
	}

	return nil
}

// create a file in our data dir. don't forget to close it!
func (cli *CLI) DataFileCreate(fpath []string, overwrite bool) (*os.File, error) {
	ddpath := cli.DataFilePath(fpath)
	if !overwrite {
		exists, err := cli.DataFileExists(fpath)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("refusing to overwrite file that exists: %s", ddpath)
		}
	}
	if len(fpath) > 1 {
		dirs, _ := filepath.Split(ddpath)
		err := os.MkdirAll(dirs, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error creating subdirectories for %s: %w", ddpath, err)
		}
	}
	return os.Create(ddpath)
}

// get a path to a segment file in our database
func (cli *CLI) SegmentFilePath(user string, file string) (string, error) {
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)
	aqt, err := aqtime.FromString(base)
	if err != nil {
		return "", err
	}
	fname := fmt.Sprintf("%s%s", aqt.FileSafeString(), ext)
	yr, mon, day, hr, min, _, _ := aqt.Parts()
	return cli.DataFilePath([]string{SegmentsDir, user, yr, mon, day, hr, min, fname}), nil
}

// get a path to a segment file in our database
func (cli *CLI) HLSDir(user string) (string, error) {
	return cli.DataFilePath([]string{SegmentsDir, "hls", user}), nil
}

// create a segment file in our database
func (cli *CLI) SegmentFileCreate(user string, aqt aqtime.AQTime, ext string) (*os.File, error) {
	fname := fmt.Sprintf("%s.%s", aqt.FileSafeString(), ext)
	yr, mon, day, hr, min, _, _ := aqt.Parts()
	return cli.DataFileCreate([]string{SegmentsDir, user, yr, mon, day, hr, min, fname}, false)
}

// ThumbnailFilePath returns the path to a user's current thumbnail. There is a
// single, continually-overwritten thumbnail per user. The user is a DID
// (e.g. did:plc:...); DataFilePath strips the colons so the filename is safe on
// Windows.
func (cli *CLI) ThumbnailFilePath(user string) string {
	return cli.DataFilePath([]string{ThumbnailsDir, fmt.Sprintf("%s.jpg", user)})
}

// ThumbnailModTime returns the modification time of a user's thumbnail and
// whether it exists. The mod time doubles as a "last seen live" signal.
func (cli *CLI) ThumbnailModTime(user string) (time.Time, bool) {
	fi, err := os.Stat(cli.ThumbnailFilePath(user))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}

// ThumbnailWrite atomically (re)writes a user's thumbnail. The image is written
// to a temp file via the supplied function and renamed into place, so readers
// (and PDS uploads) never observe a half-written thumbnail.
func (cli *CLI) ThumbnailWrite(user string, write func(io.Writer) error) error {
	final := cli.ThumbnailFilePath(user)
	dir := filepath.Dir(final)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating thumbnail dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "thumb-*.jpg")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below succeeds
	if err := write(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), final)
}

// read a file from our data dir
func (cli *CLI) DataFileRead(fpath []string, w io.Writer) error {
	ddpath := cli.DataFilePath(fpath)

	fd, err := os.Open(ddpath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, fd)
	if err != nil {
		return err
	}

	return nil
}

func (cli *CLI) HasMist() bool {
	return runtime.GOOS == "linux"
}

// type for comma-separated ethereum addresses
func (cli *CLI) AddressSliceFlag(name, defaultValue, usage string, dest *[]aqpub.Pub) urfavecli.Flag {
	*dest = []aqpub.Pub{}
	usage = fmt.Sprintf(`%s (default: "%s")`, usage, defaultValue)

	return &urfavecli.StringFlag{
		Name:  name,
		Usage: usage,
		Action: func(ctx context.Context, cmd *urfavecli.Command, s string) error {
			if s == "" {
				return nil
			}
			strs := strings.Split(s, ",")
			for _, str := range strs {
				pub, err := aqpub.FromHexString(str)
				if err != nil {
					return err
				}
				*dest = append(*dest, pub)
			}
			return nil
		},
		Sources: urfavecli.EnvVars(fmt.Sprintf("SP_%s", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))),
	}
}

func (cli *CLI) StreamIsAllowed(did string) error {
	if cli.WideOpen {
		return nil
	}
	// if the user set no test streams, anyone can stream
	openServer := len(cli.AllowedStreams) == 0 || (cli.TestStream && len(cli.AllowedStreams) == 1)
	// but only valid atproto accounts! did:key is only allowed for our local test stream
	isDIDKey := strings.HasPrefix(did, constants.DID_KEY_PREFIX)
	if openServer && !isDIDKey {
		return nil
	}
	if slices.Contains(cli.AllowedStreams, did) {
		return nil
	}
	return fmt.Errorf("user is not allowed to stream")
}

func (cli *CLI) BroadcasterDID() string {
	return fmt.Sprintf("did:web:%s", cli.BroadcasterHost)
}

func (cli *CLI) ServerDID() string {
	if cli.ServerHost == "" {
		return cli.BroadcasterDID()
	}
	return fmt.Sprintf("did:web:%s", cli.ServerHost)
}

func (cli *CLI) HasHTTPS() bool {
	return cli.Secure || cli.BehindHTTPSProxy
}

func (cli *CLI) DumpDebugSegment(ctx context.Context, name string, r io.Reader) {
	if cli.SegmentDebugDir == "" {
		return
	}
	go func() {
		err := os.MkdirAll(cli.SegmentDebugDir, 0755)
		if err != nil {
			log.Error(ctx, "failed to create debug directory", "error", err)
			return
		}
		now := aqtime.FromTime(time.Now())
		outFile := filepath.Join(cli.SegmentDebugDir, fmt.Sprintf("%s-%s", now.FileSafeString(), strings.ReplaceAll(name, ":", "-")))
		fd, err := os.Create(outFile)
		if err != nil {
			log.Error(ctx, "failed to create debug file", "error", err)
			return
		}
		defer fd.Close()
		_, err = io.Copy(fd, r)
		if err != nil {
			log.Error(ctx, "failed to copy debug file", "error", err)
			return
		}
		log.Log(ctx, "wrote debug file", "path", outFile)
	}()
}

func (cli *CLI) S3Configured() bool {
	return cli.S3Endpoint != "" && cli.S3Bucket != "" && cli.S3AccessKeyID != "" && cli.S3SecretAccessKey != ""
}

// S3Config assembles an s3.Config from the CLI's S3 flags.
func (cli *CLI) S3Config() s3.Config {
	return s3.Config{
		Endpoint:        cli.S3Endpoint,
		Bucket:          cli.S3Bucket,
		AccessKeyID:     cli.S3AccessKeyID,
		SecretAccessKey: cli.S3SecretAccessKey,
		Region:          cli.S3Region,
	}
}

// SetS3Config applies an s3.Config to the CLI's S3 fields — the inverse of
// S3Config, for processes (ingest workers) that receive the S3 destination over
// a handshake instead of from flags.
func (cli *CLI) SetS3Config(c s3.Config) {
	cli.S3Endpoint = c.Endpoint
	cli.S3Bucket = c.Bucket
	cli.S3AccessKeyID = c.AccessKeyID
	cli.S3SecretAccessKey = c.SecretAccessKey
	cli.S3Region = c.Region
}

// DebugRecordingFile is the write target returned by DebugRecordingCreate: an
// *os.File on local disk, or an S3 upload that commits on Close. Name() reports
// the destination (path or object key) for logging.
type DebugRecordingFile interface {
	io.WriteCloser
	Name() string
}

// DebugRecordingCreate opens a write target for a debug recording (RTMP/MKV
// dumps, WHIP rtcrec sessions). When S3 is configured the recording streams to
// an S3 object at the key formed by joining fpath with "/" (so the bucket
// mirrors the on-disk debug-recordings/<did>/<file> layout); otherwise it falls
// back to a local file under DataDir — the dev default. The returned value must
// be Closed to finalize (Close commits the S3 upload). overwrite only affects
// the local-disk path (S3 puts always overwrite).
func (cli *CLI) DebugRecordingCreate(ctx context.Context, fpath []string, contentType string, overwrite bool) (DebugRecordingFile, error) {
	if cli.S3Configured() {
		key := strings.Join(fpath, "/")
		// The recording outlives the ingest session's ctx: Close commits the upload
		// during teardown, after that ctx is typically cancelled — a cancelled ctx
		// here would abort the upload and lose the object. Callers bound the commit
		// with their own finalize waits instead.
		return s3.NewUploadWriter(context.WithoutCancel(ctx), s3.NewClient(cli.S3Config()), cli.S3Bucket, key, contentType)
	}
	return cli.DataFileCreate(fpath, overwrite)
}

func (cli *CLI) ShouldSyndicate(did string) bool {
	if cli.DisableSyndication {
		return false
	}
	for _, d := range cli.Syndicate {
		if d == "*" {
			return true
		}
		if d == did {
			return true
		}
	}
	return false
}
