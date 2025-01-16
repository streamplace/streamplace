package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
)

func TestRegexStreamplace(t *testing.T) {
	tests := []struct {
		filename       string
		shouldMatch    bool
		expectedGroups []string
	}{
		// Test cases for the 're' regex
		{"streamplace-v1.2.3-abcdef-foo-bar.txt", true, []string{"", "v1.2.3", "-abcdef", "foo", "bar", "", "txt"}},
		{"streamplace-v1.0.0-123456-hello-world.csv", true, []string{"", "v1.0.0", "-123456", "hello", "world", "", "csv"}},
		{"streamplace-v2.5.1-abc123-done-done.xml", true, []string{"", "v2.5.1", "-abc123", "done", "done", "", "xml"}},
		{"streamplace-v3.2.1-xyz-abc.json", true, []string{"", "v3.2.1", "", "xyz", "abc", "", "json"}},
		{"streamplace-v3.2.1-nohash-xyz.json", true, []string{"", "v3.2.1", "", "nohash", "xyz", "", "json"}},
		{"streamplace-v10.2.10-abc123-linux-amd64.json", true, []string{"", "v10.2.10", "-abc123", "linux", "amd64", "", "json"}},
		{"streamplace-v10.2.10-darwin-arm64.json", true, []string{"", "v10.2.10", "", "darwin", "arm64", "", "json"}},
		{"streamplace-desktop-v3.2.1-nohash-xyz.json", true, []string{"-desktop", "v3.2.1", "", "nohash", "xyz", "", "json"}},
		{"streamplace-desktop-v10.2.10-abc123-linux-amd64.json", true, []string{"-desktop", "v10.2.10", "-abc123", "linux", "amd64", "", "json"}},
		{"streamplace-desktop-v10.2.10-darwin-arm64.json", true, []string{"-desktop", "v10.2.10", "", "darwin", "arm64", "", "json"}},
		{"streamplace-desktop-v10.2.10-darwin-arm64.json", true, []string{"-desktop", "v10.2.10", "", "darwin", "arm64", "", "json"}},
		{"streamplace-desktop-v0.1.3-5742a5a4-windows-amd64.1cbc2208decb3e55c7aea7320258aa36e3297f18.nupkg", true, []string{"-desktop", "v0.1.3", "-5742a5a4", "windows", "amd64", "1cbc2208decb3e55c7aea7320258aa36e3297f18", "nupkg"}},

		// Test cases where the regex should not match
		{"streamplace-123-abc.txt", false, nil},
		{"streamplace-v1.2.3-abc.txt", false, nil},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			match := re.FindStringSubmatch(test.filename)
			if test.shouldMatch {
				require.NotNil(t, match, "Expected match for filename %s", test.filename)
				require.Len(t, match, len(test.expectedGroups)+1, "Unexpected number of capture groups for filename %s", test.filename)
				for i, expected := range test.expectedGroups {
					require.Equal(t, expected, match[i+1], "Unexpected group %d for filename %s", i, test.filename)
				}
			} else {
				require.Nil(t, match, "Expected no match for filename %s", test.filename)
			}
		})
	}
}

func TestRegexInput(t *testing.T) {
	tests := []struct {
		filename       string
		shouldMatch    bool
		expectedGroups []string
	}{
		// Test cases for the 'inputRe' regex
		{"streamplace-foo-bar.txt", true, []string{"", "foo", "bar", "txt"}},
		{"streamplace-abc-def.csv", true, []string{"", "abc", "def", "csv"}},
		{"streamplace-x-y.xml", true, []string{"", "x", "y", "xml"}},
		{"streamplace-hello-world.json", true, []string{"", "hello", "world", "json"}},
		{"streamplace-desktop-x-y.xml", true, []string{"-desktop", "x", "y", "xml"}},
		{"streamplace-desktop-hello-world.json", true, []string{"-desktop", "hello", "world", "json"}},

		// Test cases where the regex should not match
		{"streamplace-foo.txt", false, nil},
		{"streamplace-foo-bar-baz.txt", false, nil},
		{"streamplace-foo-bar-baz-qux.txt", false, nil},
		{"streamplacefoo-bar.txt", false, nil},
		{"streamplace-foo-bar.", false, nil},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			match := inputRe.FindStringSubmatch(test.filename)
			if test.shouldMatch {
				require.NotNil(t, match, "Expected match for filename %s", test.filename)
				require.Len(t, match, len(test.expectedGroups)+1, "Unexpected number of capture groups for filename %s", test.filename)
				for i, expected := range test.expectedGroups {
					require.Equal(t, expected, match[i+1], "Unexpected group %d for filename %s", i, test.filename)
				}
			} else {
				require.Nil(t, match, "Expected no match for filename %s", test.filename)
			}
		})
	}
}

func TestDownloadRedirects(t *testing.T) {
	branch := "electron"
	cli := &config.CLI{GitLabURL: "https://example.com/api/v4/projects/173"}
	queryGitlab = func(url string) (io.ReadCloser, error) {
		pkgUrl := fmt.Sprintf("%s/packages?order_by=created_at&sort=desc&package_name=%s", cli.GitLabURL, branch)
		fileUrl := fmt.Sprintf("%s/packages/339/package_files", cli.GitLabURL)
		var bs []byte
		if url == pkgUrl {
			bs = packageRes
		} else if url == fileUrl {
			bs = fileRes
		} else {
			return nil, fmt.Errorf("unknown url: '%s'  (wanted '%s' or '%s')", url, pkgUrl, fileUrl)
		}
		r := bytes.NewReader(bs)
		return io.NopCloser(r), nil
	}
	defer func() { queryGitlab = queryGitlabReal }()
	tests := []struct {
		in  string
		out string
	}{
		{
			in:  "streamplace-linux-amd64.tar.gz",
			out: "v0.1.3-51aab8b5/streamplace-v0.1.3-51aab8b5-linux-amd64.tar.gz",
		},
		{
			in:  "streamplace-linux-arm64.tar.gz",
			out: "v0.1.3-51aab8b5/streamplace-v0.1.3-51aab8b5-linux-arm64.tar.gz",
		},
		{
			in:  "streamplace-darwin-amd64.tar.gz",
			out: "v0.1.3-51aab8b5/streamplace-v0.1.3-51aab8b5-darwin-amd64.tar.gz",
		},
		{
			in:  "streamplace-darwin-arm64.tar.gz",
			out: "v0.1.3-51aab8b5/streamplace-v0.1.3-51aab8b5-darwin-arm64.tar.gz",
		},
		{
			in:  "streamplace-windows-amd64.zip",
			out: "v0.1.3-51aab8b5/streamplace-v0.1.3-51aab8b5-windows-amd64.zip",
		},
		{
			in:  "streamplace-desktop-windows-amd64.exe",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-windows-amd64.exe",
		},
		{
			in:  "streamplace-desktop-darwin-amd64.dmg",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-darwin-amd64.dmg",
		},
		{
			in:  "streamplace-desktop-darwin-arm64.dmg",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-darwin-arm64.dmg",
		},
		{
			in:  "streamplace-desktop-darwin-amd64.zip",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-darwin-amd64.zip",
		},
		{
			in:  "streamplace-desktop-darwin-arm64.zip",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-darwin-arm64.zip",
		},
		{
			in:  "streamplace-desktop-linux-amd64.AppImage",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-linux-amd64.AppImage",
		},
		{
			in:  "streamplace-desktop-linux-arm64.AppImage",
			out: "v0.1.3-51aab8b5/streamplace-desktop-v0.1.3-51aab8b5-linux-arm64.AppImage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			mod := &model.DBModel{}
			a := StreamplaceAPI{CLI: cli, Model: mod}

			handler := a.HandleAppDownload(context.Background())

			reqUrl := fmt.Sprintf("/dl/%s/%s", branch, tt.in)

			req := httptest.NewRequest("GET", reqUrl, nil)
			rr := httptest.NewRecorder()

			handler(rr, req, httprouter.Params{})

			result := rr.Result()
			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode, "handler returned wrong status code")

			redirectURL, err := result.Location()
			require.NoError(t, err, "Failed to get redirect location")
			fullOut := fmt.Sprintf("%s/packages/generic/%s/%s", cli.GitLabURL, branch, tt.out)

			require.Equal(t, fullOut, redirectURL.String(), "handler returned unexpected redirect URL")
		})
	}
}

var packageRes = []byte(`
	[
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/339" },
			"created_at": "2024-09-19T20:28:06.445Z",
			"id": 339,
			"last_downloaded_at": "2024-09-19T20:40:18.942Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-19T20:08:42.262Z",
				"id": 572,
				"iid": 513,
				"project_id": 1,
				"ref": "electron",
				"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
				"source": "push",
				"status": "running",
				"updated_at": "2024-09-19T20:08:45.699Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-51aab8b5"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/338" },
			"created_at": "2024-09-18T23:53:51.141Z",
			"id": 338,
			"last_downloaded_at": "2024-09-19T00:10:31.948Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-18T23:33:16.873Z",
				"id": 571,
				"iid": 512,
				"project_id": 1,
				"ref": "electron",
				"sha": "78fcaf170355c45134d67010df2caaeaf5a5facc",
				"source": "push",
				"status": "success",
				"updated_at": "2024-09-19T00:11:36.764Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/571"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-78fcaf17"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/337" },
			"created_at": "2024-09-17T20:19:55.436Z",
			"id": 337,
			"last_downloaded_at": "2024-09-17T21:04:52.491Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-17T19:26:53.407Z",
				"id": 567,
				"iid": 508,
				"project_id": 1,
				"ref": "electron",
				"sha": "4043c87ab6d19db9706ba4a3c05dac766f9e6073",
				"source": "push",
				"status": "success",
				"updated_at": "2024-09-17T21:05:57.420Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/567"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-4043c87a"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/336" },
			"created_at": "2024-09-17T04:08:57.406Z",
			"id": 336,
			"last_downloaded_at": "2024-09-17T04:34:20.263Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-17T03:49:53.430Z",
				"id": 566,
				"iid": 507,
				"project_id": 1,
				"ref": "electron",
				"sha": "a32eed6761d9375eda907aff7040699b8f18217b",
				"source": "push",
				"status": "success",
				"updated_at": "2024-09-17T04:57:57.731Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/2/avatar.png",
					"id": 2,
					"locked": false,
					"name": "Eli Mallon",
					"state": "active",
					"username": "iameli",
					"web_url": "https://git.aquareum.tv/iameli"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/566"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-a32eed67"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/335" },
			"created_at": "2024-09-17T00:18:09.095Z",
			"id": 335,
			"last_downloaded_at": "2024-09-17T03:50:09.272Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-16T23:59:10.080Z",
				"id": 565,
				"iid": 506,
				"project_id": 1,
				"ref": "electron",
				"sha": "41ee5c4cb63b490fb7b9d55f3faf828cc4c6b923",
				"source": "push",
				"status": "canceled",
				"updated_at": "2024-09-17T03:50:51.613Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/565"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-41ee5c4c"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/334" },
			"created_at": "2024-09-16T23:47:04.209Z",
			"id": 334,
			"last_downloaded_at": null,
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-16T23:27:43.205Z",
				"id": 564,
				"iid": 505,
				"project_id": 1,
				"ref": "electron",
				"sha": "fa71bac9aaebc2d54c8020ad93e5bcbd9788eefc",
				"source": "push",
				"status": "failed",
				"updated_at": "2024-09-17T00:22:35.717Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/564"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-fa71bac9"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/333" },
			"created_at": "2024-09-16T22:52:23.113Z",
			"id": 333,
			"last_downloaded_at": null,
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-16T22:33:49.464Z",
				"id": 563,
				"iid": 504,
				"project_id": 1,
				"ref": "electron",
				"sha": "54d95b19bc394500cf910a5012dd864d2ff35b20",
				"source": "push",
				"status": "failed",
				"updated_at": "2024-09-16T23:32:25.977Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/563"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-54d95b19"
		},
		{
			"_links": { "web_path": "/streamplace/streamplace/-/packages/332" },
			"created_at": "2024-09-16T20:29:27.305Z",
			"id": 332,
			"last_downloaded_at": "2024-09-16T20:40:19.669Z",
			"name": "electron",
			"package_type": "generic",
			"pipeline": {
				"created_at": "2024-09-16T20:08:13.199Z",
				"id": 561,
				"iid": 502,
				"project_id": 1,
				"ref": "electron",
				"sha": "592325bed64178e791e89301424c60f805ae3835",
				"source": "push",
				"status": "success",
				"updated_at": "2024-09-16T21:28:12.814Z",
				"user": {
					"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
					"id": 5,
					"locked": false,
					"name": "Eli Streams",
					"state": "active",
					"username": "iameli-streams",
					"web_url": "https://git.aquareum.tv/iameli-streams"
				},
				"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/561"
			},
			"pipelines": [],
			"status": "default",
			"tags": [],
			"version": "v0.1.3-592325be"
		}
	]
`)

var fileRes = []byte(`
	[
		{
			"created_at": "2024-09-19T20:28:06.468Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-ios-release.xcarchive.tar.gz",
			"file_sha1": null,
			"file_sha256": "d4d88c885f1494e3698ac24923a2b1d1e632a6c936d039f392e927fe1c4e8fa0",
			"id": 1994,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 44092840
		},
		{
			"created_at": "2024-09-19T20:29:59.539Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-android-release.apk",
			"file_sha1": null,
			"file_sha256": "d409d179f7fa5eb7834607a76a845681d0358b8c1652d2a6206a967a0a1e4a07",
			"id": 1995,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 77485100
		},
		{
			"created_at": "2024-09-19T20:30:00.386Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-darwin-amd64.tar.gz",
			"file_sha1": null,
			"file_sha256": "c019a776e3e43b0fc40bdbda972ac6bc98ed1df54016ce4510452044175ab1fb",
			"id": 1996,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 35380619
		},
		{
			"created_at": "2024-09-19T20:30:03.952Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-android-debug.apk",
			"file_sha1": null,
			"file_sha256": "4fdef8e9afcc0b50971595470e5373de7e71e49c909b53729d881c1d11998e64",
			"id": 1997,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 165891665
		},
		{
			"created_at": "2024-09-19T20:30:06.333Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-darwin-amd64.dmg",
			"file_sha1": null,
			"file_sha256": "c397c9e689e5aa14e4e608d6b8bfe684aaa80d0fd96e65c2240a23f1bbe2584e",
			"id": 1998,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 138530555
		},
		{
			"created_at": "2024-09-19T20:30:06.515Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-android-release.aab",
			"file_sha1": null,
			"file_sha256": "b76828177c0dd02cfe80a763034acdbfd6cedd70e5afd4eefa3221acf7170d75",
			"id": 1999,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 39472720
		},
		{
			"created_at": "2024-09-19T20:30:09.006Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-android-debug.aab",
			"file_sha1": null,
			"file_sha256": "0bcf65f37ca70d2f3d904d4eedcb1dc4e162901fe3115a77eedea4aefca0b58e",
			"id": 2000,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 54428823
		},
		{
			"created_at": "2024-09-19T20:30:11.825Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-darwin-amd64.zip",
			"file_sha1": null,
			"file_sha256": "d6507feb9227b374b15eaf5558abdc31a652d977cc0d089352dea494399e9dcd",
			"id": 2001,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 137738479
		},
		{
			"created_at": "2024-09-19T20:30:15.385Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-darwin-arm64.tar.gz",
			"file_sha1": null,
			"file_sha256": "2e1425d550c54b52a708a97b6c545ae511bee79f04490867c63331a3560c0a87",
			"id": 2002,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 34455283
		},
		{
			"created_at": "2024-09-19T20:30:20.884Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-darwin-arm64.dmg",
			"file_sha1": null,
			"file_sha256": "92bc4e858a03330a034f2f71d7579f35491b651613050d48a548667cb2d74059",
			"id": 2003,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 132833884
		},
		{
			"created_at": "2024-09-19T20:30:26.199Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-darwin-arm64.zip",
			"file_sha1": null,
			"file_sha256": "06ab9412bdc23e48d124cc6cb032d1f21c75138965ee4c8b87b1524cedfa5318",
			"id": 2004,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 131923036
		},
		{
			"created_at": "2024-09-19T20:39:30.156Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-linux-amd64.tar.gz",
			"file_sha1": null,
			"file_sha256": "f7bf0191c5d5bd2d94533f8e3562a0f30df8a166efe97bf2d95802a7a3592054",
			"id": 2005,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 101700182
		},
		{
			"created_at": "2024-09-19T20:39:35.057Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-linux-amd64.AppImage",
			"file_sha1": null,
			"file_sha256": "15f5a5104577cd7eae2c394d897ce06622fc7a0c567dba6c270a521ebf235dcc",
			"id": 2006,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 210871488
		},
		{
			"created_at": "2024-09-19T20:39:38.495Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-linux-arm64.tar.gz",
			"file_sha1": null,
			"file_sha256": "3b5f79b11333d3a7466c2f4377ce23c1cbc612fc136fefd5ac7bba4e0729ac0d",
			"id": 2007,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 97621477
		},
		{
			"created_at": "2024-09-19T20:39:43.684Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-linux-arm64.AppImage",
			"file_sha1": null,
			"file_sha256": "82dff1e67de9796879e6a9573257c1f83f71326bf0ba4f33dec97d536a61f31c",
			"id": 2008,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 207205568
		},
		{
			"created_at": "2024-09-19T20:39:46.855Z",
			"file_md5": null,
			"file_name": "streamplace-v0.1.3-51aab8b5-windows-amd64.zip",
			"file_sha1": null,
			"file_sha256": "e420f638ddc7f6352ea78a30a8efab12672e8840158d2fba04a8cf448a8edaa1",
			"id": 2009,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 67260033
		},
		{
			"created_at": "2024-09-19T20:39:51.454Z",
			"file_md5": null,
			"file_name": "streamplace-desktop-v0.1.3-51aab8b5-windows-amd64.exe",
			"file_sha1": null,
			"file_sha256": "b0b44dfb460b3faf7dd13f97df9f91815b865283bb92d810304b537cbabaa2b1",
			"id": 2010,
			"package_id": 339,
			"pipelines": [
				{
					"created_at": "2024-09-19T20:08:42.262Z",
					"id": 572,
					"iid": 513,
					"project_id": 1,
					"ref": "electron",
					"sha": "51aab8b5ef805a01543c1b7d0646a937905b6597",
					"source": "push",
					"status": "running",
					"updated_at": "2024-09-19T20:08:45.699Z",
					"user": {
						"avatar_url": "https://git.aquareum.tv/uploads/-/system/user/avatar/5/avatar.png",
						"id": 5,
						"locked": false,
						"name": "Eli Streams",
						"state": "active",
						"username": "iameli-streams",
						"web_url": "https://git.aquareum.tv/iameli-streams"
					},
					"web_url": "https://git.aquareum.tv/streamplace/streamplace/-/pipelines/572"
				}
			],
			"size": 175016960
		}
	]
`)
