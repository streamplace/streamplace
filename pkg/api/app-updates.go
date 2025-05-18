package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"

	"github.com/dunglas/httpsfv"
)

const IOS = "ios"
const ANDROID = "android"

type UpdateManifest struct {
	ID             string            `json:"id"`
	CreatedAt      string            `json:"createdAt"`
	RuntimeVersion string            `json:"runtimeVersion"`
	LaunchAsset    UpdateAsset       `json:"launchAsset"`
	Assets         []UpdateAsset     `json:"assets"`
	Metadata       map[string]string `json:"metadata"`
	Extra          map[string]any    `json:"extra"`
}

type UpdateAsset struct {
	Hash          string `json:"hash,omitempty"`
	Key           string `json:"key"`
	ContentType   string `json:"contentType"`
	FileExtension string `json:"fileExtension,omitempty"`
	URL           string `json:"url"`
	Path          string `json:"-"`
}

type ExpoMetadata struct {
	Version      int    `json:"version"`
	Bundler      string `json:"bundler"`
	FileMetadata struct {
		IOS     ExpoMetadataPlatform `json:"ios"`
		Android ExpoMetadataPlatform `json:"android"`
	} `json:"fileMetadata"`
}

type ExpoMetadataPlatform struct {
	Bundle string `json:"bundle"`
	Assets []struct {
		Path string `json:"path"`
		Ext  string `json:"ext"`
	} `json:"assets"`
}

type Updater struct {
	Metadata       ExpoMetadata
	Extra          map[string]any
	CLI            *config.CLI
	SigningKey     *rsa.PrivateKey
	RuntimeVersion string
}

func (u *Updater) GetManifest(platform, runtime, prefix string) (*UpdateManifest, error) {
	var plat ExpoMetadataPlatform
	if platform == IOS {
		plat = u.Metadata.FileMetadata.IOS
	} else if platform == ANDROID {
		plat = u.Metadata.FileMetadata.Android
	} else {
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}
	assets := []UpdateAsset{}
	for _, ass := range plat.Assets {
		ext := fmt.Sprintf(".%s", ass.Ext)
		loadEmbeddedMimes()
		typ := mime.TypeByExtension(ext)

		if typ == "" {
			return nil, fmt.Errorf("unknown content-type for file extention %s", ext)
		}
		parts := strings.Split(ass.Path, "/")
		hash, err := hashFile(ass.Path)
		if err != nil {
			return nil, err
		}
		assets = append(assets, UpdateAsset{
			Hash:          hash,
			Key:           parts[len(parts)-1],
			Path:          ass.Path,
			URL:           fmt.Sprintf("%s/%s", prefix, ass.Path),
			ContentType:   typ,
			FileExtension: ass.Ext,
		})
	}
	dashParts := strings.Split(plat.Bundle, "-")
	dotParts := strings.Split(dashParts[len(dashParts)-1], ".")
	hash, err := hashFile(plat.Bundle)
	if err != nil {
		return nil, err
	}
	man := UpdateManifest{
		ID:             u.CLI.Build.UUID,
		CreatedAt:      u.CLI.Build.BuildTimeStrExpo(),
		RuntimeVersion: runtime,
		LaunchAsset: UpdateAsset{
			Hash:        hash,
			Key:         dotParts[0],
			Path:        plat.Bundle,
			URL:         fmt.Sprintf("%s/%s", prefix, plat.Bundle),
			ContentType: "application/javascript",
		},
		Assets:   assets,
		Metadata: map[string]string{},
		Extra:    u.Extra,
	}
	return &man, nil
}

var DefaultKey = "main"

// get keyid, with a default if there's not one
func getKeyID(header string) string {
	d, err := httpsfv.UnmarshalDictionary([]string{header})
	if err != nil {
		return DefaultKey
	}
	key, ok := d.Get("keyid")
	if !ok {
		return DefaultKey
	}
	keystr, ok := key.(httpsfv.Item).Value.(string)
	if !ok {
		return DefaultKey
	}
	return keystr
}

func (u *Updater) GetManifestBytes(platform, runtime, signing, prefix string) ([]byte, string, error) {
	if runtime != u.RuntimeVersion {
		return nil, "", fmt.Errorf("runtime version mismatch client=%s server=%s", runtime, u.RuntimeVersion)
	}
	manifest, err := u.GetManifest(platform, runtime, prefix)
	if err != nil {
		return nil, "", err
	}
	bs, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	var header string
	if u.SigningKey != nil {
		keyid := getKeyID(signing)
		msgHash := sha256.New()
		_, err = msgHash.Write(bs)
		if err != nil {
			return nil, "", fmt.Errorf("error getting sha256 hash of manifest: %w", err)
		}
		msgHashSum := msgHash.Sum(nil)
		signature, err := rsa.SignPKCS1v15(rand.Reader, u.SigningKey, crypto.SHA256, msgHashSum)
		if err != nil {
			return nil, "", fmt.Errorf("error signing manifest: %w", err)
		}
		sigString := base64.StdEncoding.EncodeToString(signature)
		dict := httpsfv.NewDictionary()
		dict.Add("sig", httpsfv.NewItem(sigString))
		dict.Add("keyid", httpsfv.NewItem(keyid))
		header, err = httpsfv.Marshal(dict)
		if err != nil {
			return nil, "", fmt.Errorf("error marshalling dict: %w", err)
		}
	}
	return bs, header, nil
}

// get MIME types of built-in update files
func (u *Updater) GetMimes() (map[string]string, error) {
	assets := []UpdateAsset{}
	ios, err := u.GetManifest(IOS, "", "")
	if err != nil {
		return nil, err
	}
	assets = append(assets, ios.LaunchAsset)
	assets = append(assets, ios.Assets...)
	android, err := u.GetManifest(ANDROID, "", "")
	if err != nil {
		return nil, err
	}
	assets = append(assets, android.LaunchAsset)
	assets = append(assets, android.Assets...)
	m := map[string]string{}
	for _, ass := range assets {
		if ass.Path == "" {
			return nil, fmt.Errorf("asset has no path! asset=%v", ass)
		}
		m[ass.Path] = ass.ContentType
	}
	return m, nil
}

func PrepareUpdater(cli *config.CLI) (*Updater, error) {
	fs, err := app.Files()
	if err != nil {
		return nil, err
	}
	file, err := fs.Open("metadata.json")
	if err != nil {
		return nil, fmt.Errorf("couldn't read metadata.json, did you run `make app`? error=%w", err)
	}
	bs, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	metadata := ExpoMetadata{}
	err = json.Unmarshal(bs, &metadata)
	if err != nil {
		return nil, err
	}

	extra, err := app.PackageJSON()
	if err != nil {
		return nil, fmt.Errorf("package.json failed")
	}

	rt, ok := extra["runtimeVersion"]
	if !ok {
		return nil, fmt.Errorf("package.json missing runtimeVersion")
	}
	runtimeVersion, ok := rt.(string)
	if !ok {
		return nil, fmt.Errorf("package.json has runtimeVersion that's not a string")
	}

	var privateKey *rsa.PrivateKey
	if cli.SigningKeyPath != "" {
		privateKey, err = cli.ParseSigningKey()
		if err != nil {
			return nil, err
		}
	}

	return &Updater{
		CLI:            cli,
		Metadata:       metadata,
		Extra:          extra,
		SigningKey:     privateKey,
		RuntimeVersion: runtimeVersion,
	}, nil
}

func (a *StreamplaceAPI) HandleAppUpdates(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		prefix := fmt.Sprintf("http://%s", req.Host)
		if req.TLS != nil {
			prefix = fmt.Sprintf("https://%s", req.Host)
		}
		log.Log(ctx, "got app-updates request", "method", req.Method, "headers", req.Header)
		plat := req.Header.Get("expo-platform")
		if plat == "" {
			log.Log(ctx, "app-updates request missing Expo-Platform")
			w.WriteHeader(400)
			return
		}
		runtime := req.Header.Get("expo-runtime-version")
		if runtime == "" {
			log.Log(ctx, "app-updates request missing Expo-Runtime-Version")
			w.WriteHeader(400)
			return
		}
		signing := req.Header.Get("expo-expect-signature")
		if signing != "" {
			if a.Updater.SigningKey == nil {
				log.Log(ctx, "signing requested but we don't have a key", "expo-expect-signature", signing)
				w.WriteHeader(501)
				return
			}
		}
		bs, header, err := a.Updater.GetManifestBytes(plat, runtime, signing, prefix)
		if err != nil {
			log.Log(ctx, "app-updates request errored getting manfiest", "error", err)
			w.WriteHeader(400)
			return
		}
		if signing != "" {
			w.Header().Set("expo-signature", header)
		}
		w.Header().Set("content-type", "application/json")
		w.Header().Set("expo-protocol-version", "1")
		w.Header().Set("expo-sfv-version", "0")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func hashFile(path string) (string, error) {
	fs, err := app.Files()
	if err != nil {
		return "", err
	}
	file, err := fs.Open(path)
	if err != nil {
		return "", err
	}
	bs, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	h := sha256.New()

	h.Write(bs)

	outbs := h.Sum(nil)

	sEnc := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(outbs)
	return sEnc, nil
}
