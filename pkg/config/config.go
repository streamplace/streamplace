package config

import (
	"crypto/rsa"
	"crypto/x509"
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

	"github.com/peterbourgon/ff/v3"
	"golang.org/x/exp/rand"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/aqpub"
)

const SP_DATA_DIR = "$SP_DATA_DIR"
const SEGMENTS_DIR = "segments"

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
	DBPath                 string
	EthAccountAddr         string
	EthKeystorePath        string
	EthPassword            string
	FirebaseServiceAccount string
	GitLabURL              string
	HttpAddr               string
	HttpInternalAddr       string
	HttpsAddr              string
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
	WHIPTest               string

	dataDirFlags []*string
}

var STREAMPLACE_SCHEME_PREFIX = "streamplace://"

func (cli *CLI) OwnInternalURL() string {
	//  No errors because we know it's valid from AddrFlag
	host, port, _ := net.SplitHostPort(cli.HttpInternalAddr)
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
		res[i] = charset[rand.Intn(len(charset))]
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

func (cli *CLI) Parse(fs *flag.FlagSet, args []string) error {
	err := ff.Parse(
		fs, os.Args[1:],
		ff.WithEnvVarPrefix("SP"),
	)
	if err != nil {
		return err
	}
	if cli.DataDir == "" {
		return fmt.Errorf("could not determine default data dir (no $HOME) and none provided, please set --data-dir")
	}
	for _, dest := range cli.dataDirFlags {
		*dest = strings.Replace(*dest, SP_DATA_DIR, cli.DataDir, 1)
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
	return cli.DataFilePath([]string{SEGMENTS_DIR, user, yr, mon, day, hr, min, fname}), nil
}

// get a path to a segment file in our database
func (cli *CLI) HLSDir(user string) (string, error) {
	return cli.DataFilePath([]string{SEGMENTS_DIR, "hls", user}), nil
}

// create a segment file in our database
func (cli *CLI) SegmentFileCreate(user string, aqt aqtime.AQTime, ext string) (*os.File, error) {
	fname := fmt.Sprintf("%s.%s", aqt.FileSafeString(), ext)
	yr, mon, day, hr, min, _, _ := aqt.Parts()
	return cli.DataFileCreate([]string{SEGMENTS_DIR, user, yr, mon, day, hr, min, fname}, false)
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
	*dest = filepath.Join(SP_DATA_DIR, defaultValue)
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
