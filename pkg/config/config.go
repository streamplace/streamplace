package config

import (
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
	"strconv"
	"strings"
	"time"

	"math/rand/v2"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/peterbourgon/ff/v3"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
)

const SPDataDir = "$SP_DATA_DIR"
const SegmentsDir = "segments"

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
	AdminAccount           string
	Build                  *BuildFlags
	DataDir                string
	DBURL                  string
	EthAccountAddr         string
	EthKeystorePath        string
	EthPassword            string
	FirebaseServiceAccount string
	GitLabURL              string
	HTTPAddr               string
	HTTPInternalAddr       string
	HTTPSAddr              string
	RtmpsAddr              string
	Secure                 bool
	NoMist                 bool
	MistAdminPort          int
	MistHTTPPort           int
	MistRTMPPort           int
	SigningKeyPath         string
	TAURL                  string
	TLSCertPath            string
	TLSKeyPath             string
	PKCS11ModulePath       string
	PKCS11Pin              string
	PKCS11TokenSlot        string
	PKCS11TokenLabel       string
	PKCS11TokenSerial      string
	PKCS11KeypairLabel     string
	PKCS11KeypairID        string
	StreamerName           string
	RelayHost              string
	Debug                  map[string]map[string]int
	AllowedStreams         []string
	WideOpen               bool
	Peers                  []string
	Redirects              []string
	TestStream             bool
	FrontendProxy          string
	AppBundleID            string
	NoFirehose             bool
	PrintChat              bool
	Color                  string
	LivepeerGatewayURL     string
	LivepeerGateway        bool
	WHIPTest               string
	Thumbnail              bool
	SmearAudio             bool
	ExternalSigning        bool
	RTMPServerAddon        string
	TracingEndpoint        string
	PublicHost             string
	RateLimitPerSecond     int
	RateLimitBurst         int
	RateLimitWebsocket     int
	JWK                    jwk.Key
	AccessJWK              jwk.Key
	dataDirFlags           []*string
	DiscordWebhooks        []*discordtypes.Webhook
	NewWebRTCPlayback      bool
	AppleTeamID            string
	AndroidCertFingerprint string
	Labelers               []string
	AtprotoDID             string
	LivepeerHelp           bool
	PLCURL                 string
	SQLLogging             bool
	SentryDSN              string
	LivepeerDebug          bool
}

func (cli *CLI) NewFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("streamplace", flag.ExitOnError)
	fs.StringVar(&cli.DataDir, "data-dir", DefaultDataDir(), "directory for keeping all streamplace data")
	fs.StringVar(&cli.HTTPAddr, "http-addr", ":38080", "Public HTTP address")
	fs.StringVar(&cli.HTTPInternalAddr, "http-internal-addr", "127.0.0.1:39090", "Private, admin-only HTTP address")
	fs.StringVar(&cli.HTTPSAddr, "https-addr", ":38443", "Public HTTPS address")
	fs.BoolVar(&cli.Secure, "secure", false, "Run with HTTPS. Required for WebRTC output")
	cli.DataDirFlag(fs, &cli.TLSCertPath, "tls-cert", filepath.Join("tls", "tls.crt"), "Path to TLS certificate")
	cli.DataDirFlag(fs, &cli.TLSKeyPath, "tls-key", filepath.Join("tls", "tls.key"), "Path to TLS key")
	fs.StringVar(&cli.SigningKeyPath, "signing-key", "", "Path to signing key for pushing OTA updates to the app")
	fs.StringVar(&cli.DBURL, "db-url", "sqlite://$SP_DATA_DIR/state.sqlite", "URL of the database to use for storing private streamplace state")
	cli.dataDirFlags = append(cli.dataDirFlags, &cli.DBURL)
	fs.StringVar(&cli.AdminAccount, "admin-account", "", "ethereum account that administrates this streamplace node")
	fs.StringVar(&cli.FirebaseServiceAccount, "firebase-service-account", "", "JSON string of a firebase service account key")
	fs.StringVar(&cli.GitLabURL, "gitlab-url", "https://git.stream.place/api/v4/projects/1", "gitlab url for generating download links")
	cli.DataDirFlag(fs, &cli.EthKeystorePath, "eth-keystore-path", "keystore", "path to ethereum keystore")
	fs.StringVar(&cli.EthAccountAddr, "eth-account-addr", "", "ethereum account address to use (if keystore contains more than one)")
	fs.StringVar(&cli.EthPassword, "eth-password", "", "password for encrypting keystore")
	fs.StringVar(&cli.TAURL, "ta-url", "http://timestamp.digicert.com", "timestamp authority server for signing")
	fs.StringVar(&cli.PKCS11ModulePath, "pkcs11-module-path", "", "path to a PKCS11 module for HSM signing, for example /usr/lib/x86_64-linux-gnu/opensc-pkcs11.so")
	fs.StringVar(&cli.PKCS11Pin, "pkcs11-pin", "", "PIN for logging into PKCS11 token. if not provided, will be prompted interactively")
	fs.StringVar(&cli.PKCS11TokenSlot, "pkcs11-token-slot", "", "slot number of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11TokenLabel, "pkcs11-token-label", "", "label of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11TokenSerial, "pkcs11-token-serial", "", "serial number of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11KeypairLabel, "pkcs11-keypair-label", "", "label of signing keypair on PKCS11 token")
	fs.StringVar(&cli.PKCS11KeypairID, "pkcs11-keypair-id", "", "id of signing keypair on PKCS11 token")
	fs.StringVar(&cli.AppBundleID, "app-bundle-id", "", "bundle id of an app that we facilitate oauth login for")
	fs.StringVar(&cli.StreamerName, "streamer-name", "", "name of the person streaming from this streamplace node")
	fs.StringVar(&cli.FrontendProxy, "dev-frontend-proxy", "", "(FOR DEVELOPMENT ONLY) proxy frontend requests to this address instead of using the bundled frontend")
	fs.StringVar(&cli.LivepeerGatewayURL, "livepeer-gateway-url", "", "URL of the Livepeer Gateway to use for transcoding")
	fs.BoolVar(&cli.LivepeerGateway, "livepeer-gateway", false, "enable embedded Livepeer Gateway")
	fs.BoolVar(&cli.WideOpen, "wide-open", false, "allow ALL streams to be uploaded to this node (not recommended for production)")
	cli.StringSliceFlag(fs, &cli.AllowedStreams, "allowed-streams", "", "if set, only allow these addresses or atproto DIDs to upload to this node")
	cli.StringSliceFlag(fs, &cli.Peers, "peers", "", "other streamplace nodes to replicate to")
	cli.StringSliceFlag(fs, &cli.Redirects, "redirects", "", "http 302s /path/one:/path/two,/path/three:/path/four")
	cli.DebugFlag(fs, &cli.Debug, "debug", "", "modified log verbosity for specific functions or files in form func=ToHLS:3,file=gstreamer.go:4")
	fs.BoolVar(&cli.TestStream, "test-stream", false, "run a built-in test stream on boot")
	fs.BoolVar(&cli.NoFirehose, "no-firehose", false, "disable the bluesky firehose")
	fs.BoolVar(&cli.PrintChat, "print-chat", false, "print chat messages to stdout")
	fs.StringVar(&cli.WHIPTest, "whip-test", "", "run a WHIP self-test with the given parameters")
	fs.StringVar(&cli.RelayHost, "relay-host", "wss://bsky.network", "websocket url for relay firehose")
	fs.Bool("insecure", false, "DEPRECATED, does nothing.")
	fs.StringVar(&cli.Color, "color", "", "'true' to enable colorized logging, 'false' to disable")
	fs.StringVar(&cli.PublicHost, "public-host", "", "public host for this streamplace node (excluding https:// e.g. stream.place)")
	fs.BoolVar(&cli.Thumbnail, "thumbnail", true, "enable thumbnail generation")
	fs.BoolVar(&cli.SmearAudio, "smear-audio", false, "enable audio smearing to create 'perfect' segment timestamps")
	fs.BoolVar(&cli.ExternalSigning, "external-signing", true, "enable external signing via exec (prevents potential memory leak)")
	fs.StringVar(&cli.TracingEndpoint, "tracing-endpoint", "", "gRPC endpoint to send traces to")
	fs.IntVar(&cli.RateLimitPerSecond, "rate-limit-per-second", 0, "rate limit for requests per second per ip")
	fs.IntVar(&cli.RateLimitBurst, "rate-limit-burst", 0, "rate limit burst for requests per ip")
	fs.IntVar(&cli.RateLimitWebsocket, "rate-limit-websocket", 10, "number of concurrent websocket connections allowed per ip")
	fs.StringVar(&cli.RTMPServerAddon, "rtmp-server-addon", "", "address of external RTMP server to forward streams to")
	fs.StringVar(&cli.RtmpsAddr, "rtmps-addr", ":1935", "address to listen for RTMPS connections")
	cli.JSONFlag(fs, &cli.DiscordWebhooks, "discord-webhooks", "[]", "JSON array of Discord webhooks to send notifications to")
	fs.BoolVar(&cli.NewWebRTCPlayback, "new-webrtc-playback", true, "enable new webrtc playback")
	fs.StringVar(&cli.AppleTeamID, "apple-team-id", "", "apple team id for deep linking")
	fs.StringVar(&cli.AndroidCertFingerprint, "android-cert-fingerprint", "", "android cert fingerprint for deep linking")
	cli.StringSliceFlag(fs, &cli.Labelers, "labelers", "", "did of labelers that this instance should subscribe to")
	fs.StringVar(&cli.AtprotoDID, "atproto-did", "", "atproto did to respond to on /.well-known/atproto-did (default did:web:PUBLIC_HOST)")
	fs.BoolVar(&cli.LivepeerHelp, "livepeer-help", false, "print help for livepeer flags and exit")
	fs.StringVar(&cli.PLCURL, "plc-url", "https://plc.directory", "url of the plc directory")
	fs.BoolVar(&cli.SQLLogging, "sql-logging", false, "enable sql logging")
	fs.StringVar(&cli.SentryDSN, "sentry-dsn", "", "sentry dsn for error reporting")
	fs.BoolVar(&cli.LivepeerDebug, "livepeer-debug", false, "log livepeer segments to $SP_DATA_DIR/livepeer-debug")

	lpFlags := flag.NewFlagSet("livepeer", flag.ContinueOnError)
	_ = starter.NewLivepeerConfig(lpFlags)
	lpFlags.VisitAll(func(f *flag.Flag) {
		adapted := LivepeerFlags.CamelToSnake[f.Name]
		fs.Var(f.Value, fmt.Sprintf("livepeer.%s", adapted), f.Usage)
	})

	if runtime.GOOS == "linux" {
		fs.BoolVar(&cli.NoMist, "no-mist", true, "Disable MistServer")
		fs.IntVar(&cli.MistAdminPort, "mist-admin-port", 14242, "MistServer admin port (internal use only)")
		fs.IntVar(&cli.MistRTMPPort, "mist-rtmp-port", 11935, "MistServer RTMP port (internal use only)")
		fs.IntVar(&cli.MistHTTPPort, "mist-http-port", 18080, "MistServer HTTP port (internal use only)")
	}
	return fs
}

var StreamplaceSchemePrefix = "streamplace://"

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
	// slogGorm.WithTraceAll(),
)

func (cli *CLI) Parse(fs *flag.FlagSet, args []string) error {
	err := ff.Parse(
		fs, args,
		ff.WithEnvVarPrefix("SP"),
	)
	if err != nil {
		return err
	}
	if cli.DataDir == "" {
		return fmt.Errorf("could not determine default data dir (no $HOME) and none provided, please set --data-dir")
	}
	if cli.LivepeerGateway && cli.LivepeerGatewayURL != "" {
		return fmt.Errorf("defining both livepeer-gateway and livepeer-gateway-url doesn't make sense. do you want an embedded gateway or an external one?")
	}
	if cli.LivepeerGateway {
		log.MonkeypatchStderr()
		gatewayPath := cli.DataFilePath([]string{"livepeer", "gateway"})
		err = fs.Set("livepeer.rtmp-addr", "127.0.0.1:0")
		if err != nil {
			return err
		}
		err = fs.Set("livepeer.data-dir", gatewayPath)
		if err != nil {
			return err
		}
		err = fs.Set("livepeer.gateway", "true")
		if err != nil {
			return err
		}
		httpAddrFlag := fs.Lookup("livepeer.http-addr")
		if httpAddrFlag == nil {
			return fmt.Errorf("livepeer.http-addr not found")
		}
		httpAddr := httpAddrFlag.Value.String()
		if httpAddr == "" {
			httpAddr = "127.0.0.1:8935"
			err = fs.Set("livepeer.http-addr", httpAddr)
			if err != nil {
				return err
			}
		}
		cli.LivepeerGatewayURL = fmt.Sprintf("http://%s", httpAddr)
	}
	for _, dest := range cli.dataDirFlags {
		*dest = strings.Replace(*dest, SPDataDir, cli.DataDir, 1)
	}
	if !cli.SQLLogging {
		GormLogger = slogGorm.New(
			slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
				TimeFormat: time.RFC3339,
			})),
		)
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

func (cli *CLI) DataDirFlag(fs *flag.FlagSet, dest *string, name, defaultValue, usage string) {
	cli.dataDirFlags = append(cli.dataDirFlags, dest)
	*dest = filepath.Join(SPDataDir, defaultValue)
	usage = fmt.Sprintf(`%s (default: "%s")`, usage, *dest)
	fs.Func(name, usage, func(s string) error {
		*dest = s
		return nil
	})
}

func (cli *CLI) HasMist() bool {
	return runtime.GOOS == "linux"
}

// type for comma-separated ethereum addresses
func (cli *CLI) AddressSliceFlag(fs *flag.FlagSet, dest *[]aqpub.Pub, name, defaultValue, usage string) {
	*dest = []aqpub.Pub{}
	usage = fmt.Sprintf(`%s (default: "%s")`, usage, *dest)
	fs.Func(name, usage, func(s string) error {
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
	})
}

func (cli *CLI) StringSliceFlag(fs *flag.FlagSet, dest *[]string, name, defaultValue, usage string) {
	*dest = []string{}
	usage = fmt.Sprintf(`%s (default: "%s")`, usage, *dest)
	fs.Func(name, usage, func(s string) error {
		if s == "" {
			return nil
		}
		strs := strings.Split(s, ",")
		*dest = append(*dest, strs...)
		return nil
	})
}

func (cli *CLI) JSONFlag(fs *flag.FlagSet, dest any, name, defaultValue, usage string) {
	usage = fmt.Sprintf(`%s (default: "%s")`, usage, defaultValue)
	fs.Func(name, usage, func(s string) error {
		if s == "" {
			return nil
		}
		return json.Unmarshal([]byte(s), dest)
	})
}

// debug flag for turning func=ToHLS:3,file=gstreamer.go:4 into {"func": {"ToHLS": 3}, "file": {"gstreamer.go": 4}}
func (cli *CLI) DebugFlag(fs *flag.FlagSet, dest *map[string]map[string]int, name, defaultValue, usage string) {
	*dest = map[string]map[string]int{}
	fs.Func(name, usage, func(s string) error {
		if s == "" {
			return nil
		}
		pairs := strings.Split(s, ",")
		for _, pair := range pairs {
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
			_, ok := (*dest)[selectorSplit[0]]
			if !ok {
				(*dest)[selectorSplit[0]] = map[string]int{}
			}
			(*dest)[selectorSplit[0]][selectorSplit[1]] = score
		}

		return nil
	})
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
	for _, a := range cli.AllowedStreams {
		if a == did {
			return nil
		}
	}
	return fmt.Errorf("user is not allowed to stream")
}

func (cli *CLI) MyDID() string {
	return fmt.Sprintf("did:web:%s", cli.PublicHost)
}
