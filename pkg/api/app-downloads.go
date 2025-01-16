package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"github.com/julienschmidt/httprouter"
)

var (
	re      = regexp.MustCompile(`^streamplace(-desktop)?-(v[0-9]+\.[0-9]+\.[0-9]+)(-[0-9a-f]+)?-([0-9a-z]+)-([0-9a-z]+)\.(?:([0-9a-f]+)\.)?(.+)$`)
	inputRe = regexp.MustCompile(`^streamplace(-desktop)?-([0-9a-z]+)-([0-9a-z]+)\.(.+)$`)
)

func queryGitlabReal(url string) (io.ReadCloser, error) {
	req, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return req.Body, nil
}

var queryGitlab = queryGitlabReal

func (a *StreamplaceAPI) HandleAppDownload(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		log.Log(ctx, "got here")
		pathname := r.URL.Path
		parts := strings.Split(pathname, "/")
		if len(parts) < 4 {
			apierrors.WriteHTTPBadRequest(w, "usage: /dl/latest/streamplace-linux-arm64.tar.gz", nil)
			return
		}

		_, branch, file := parts[1], parts[2], parts[3]
		if branch == "" || file == "" {
			apierrors.WriteHTTPBadRequest(w, "usage: /dl/latest/streamplace-linux-arm64.tar.gz", nil)
			return
		}

		inputPieces := inputRe.FindStringSubmatch(file)
		if inputPieces == nil {
			apierrors.WriteHTTPBadRequest(w, fmt.Sprintf("could not parse filename %s", file), nil)
			return
		}

		inputDesktop, inputPlatform, inputArch, inputExt := inputPieces[1], inputPieces[2], inputPieces[3], inputPieces[4]
		files, err := a.getGitlabPackage(branch)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, fmt.Sprintf("could not get gitlab package %s", file), err)
			return
		}

		var foundFile *GitlabFile
		for _, f := range files {
			if f.Desktop == inputDesktop && f.Platform == inputPlatform && f.Architecture == inputArch && f.Extension == inputExt {
				foundFile = &f
				break
			}
		}

		if foundFile == nil {
			apierrors.WriteHTTPNotFound(w, fmt.Sprintf("could not find a file for desktop=%s platform=%s arch=%s ext=%s", inputDesktop, inputPlatform, inputArch, inputExt), nil)
			return
		}

		http.Redirect(w, r, foundFile.URL(), http.StatusTemporaryRedirect)
	}
}

type GitlabFile struct {
	GitLabURL    string
	Branch       string
	Filename     string
	Desktop      string
	Version      string
	Hash         string
	Platform     string
	Architecture string
	SHA1         string
	Extension    string
	Size         int
}

func (f GitlabFile) FullVer() string {
	return f.Version + f.Hash
}

func (f GitlabFile) URL() string {
	return fmt.Sprintf("%s/packages/generic/%s/%s/%s", f.GitLabURL, f.Branch, f.FullVer(), f.Filename)
}

func (a *StreamplaceAPI) getGitlabPackage(branch string) ([]GitlabFile, error) {
	packageURL := fmt.Sprintf("%s/packages?order_by=created_at&sort=desc&package_name=%s", a.CLI.GitLabURL, branch)

	packageBody, err := queryGitlab(packageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages: %w", err)
	}
	defer packageBody.Close()

	var packages []map[string]any
	if err := json.NewDecoder(packageBody).Decode(&packages); err != nil {
		return nil, fmt.Errorf("failed to decode package response: %w", err)
	}
	// bs, _ := json.Marshal(packages)
	// fmt.Println(string(bs))

	if len(packages) == 0 {
		return nil, fmt.Errorf("package for branch %s not found", branch)
	}

	pkg := packages[0]
	fileURL := fmt.Sprintf("%s/packages/%v/package_files", a.CLI.GitLabURL, pkg["id"])

	fileBody, err := queryGitlab(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch files: %w", err)
	}
	defer fileBody.Close()

	var files []map[string]any
	if err := json.NewDecoder(fileBody).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode file response: %w", err)
	}

	out := []GitlabFile{}
	for _, f := range files {
		filename, ok := f["file_name"].(string)
		if !ok {
			continue
		}
		pieces := re.FindStringSubmatch(filename)
		if pieces == nil {
			// log.Log(ctx, "could not parse filename %s", "filename", filename)
			continue
		}
		size, ok := f["size"].(float64)
		if !ok {
			continue
		}
		desktop, ver, hash, platform, arch, sha1, ext := pieces[1], pieces[2], pieces[3], pieces[4], pieces[5], pieces[6], pieces[7]
		out = append(out, GitlabFile{
			GitLabURL:    a.CLI.GitLabURL,
			Branch:       branch,
			Filename:     filename,
			Desktop:      desktop,
			Version:      ver,
			Hash:         hash,
			Platform:     platform,
			Architecture: arch,
			SHA1:         sha1,
			Extension:    ext,
			Size:         int(size),
		})
	}
	return out, nil
}
