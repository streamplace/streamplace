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
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/aqtime"
	"aquareum.tv/aquareum/pkg/crypto/aqpub"
	"github.com/peterbourgon/ff/v3"
	"golang.org/x/exp/rand"
)

const AQ_DATA_DIR = "$AQ_DATA_DIR"
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
	AllowedStreams         []aqpub.Pub
	Peers                  []string
	TestStream             bool

	dataDirFlags []*string
}

var AQUAREUM_SCHEME_PREFIX = "aquareum://"

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
	return filepath.Join(home, ".aquareum")
}

func (cli *CLI) Parse(fs *flag.FlagSet, args []string) error {
	err := ff.Parse(
		fs, os.Args[1:],
		ff.WithEnvVarPrefix("AQ"),
	)
	if err != nil {
		return err
	}
	if cli.DataDir == "" {
		return fmt.Errorf("could not determine default data dir (no $HOME) and none provided, please set --data-dir")
	}
	for _, dest := range cli.dataDirFlags {
		*dest = strings.Replace(*dest, AQ_DATA_DIR, cli.DataDir, 1)
	}
	return nil
}

func (cli *CLI) dataFilePath(fpath []string) string {
	if cli.DataDir == "" {
		panic("no data dir configured")
	}
	fpath = append([]string{cli.DataDir}, fpath...)
	fdpath := filepath.Join(fpath...)
	return fdpath
}

// does a file exist in our data dir?
func (cli *CLI) DataFileExists(fpath []string) (bool, error) {
	ddpath := cli.dataFilePath(fpath)
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
	ddpath := cli.dataFilePath(fpath)
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
	return cli.dataFilePath([]string{SEGMENTS_DIR, user, yr, mon, day, hr, min, fname}), nil
}

// get a path to a segment file in our database
func (cli *CLI) HLSDir(user string) (string, error) {
	return cli.dataFilePath([]string{SEGMENTS_DIR, "hls", user}), nil
}

// create a segment file in our database
func (cli *CLI) SegmentFileCreate(user string, aqt aqtime.AQTime, ext string) (*os.File, error) {
	fname := fmt.Sprintf("%s.%s", aqt.FileSafeString(), ext)
	yr, mon, day, hr, min, _, _ := aqt.Parts()
	return cli.DataFileCreate([]string{SEGMENTS_DIR, user, yr, mon, day, hr, min, fname}, false)
}

// read a file from our data dir
func (cli *CLI) DataFileRead(fpath []string, w io.Writer) error {
	ddpath := cli.dataFilePath(fpath)

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
	*dest = filepath.Join(AQ_DATA_DIR, defaultValue)
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
