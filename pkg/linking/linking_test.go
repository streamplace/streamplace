package linking

// import (
// 	"context"
// 	"net/url"
// 	"testing"

// 	"github.com/bluesky-social/indigo/api/atproto"
// 	"github.com/bluesky-social/indigo/api/bsky"
// 	lexutil "github.com/bluesky-social/indigo/lex/util"
// 	"github.com/stretchr/testify/require"
// 	"stream.place/streamplace/pkg/streamplace"
// )

// func TestNewLinker(t *testing.T) {
// 	linker, err := NewLinker(context.Background(), []byte(input))
// 	require.NoError(t, err)
// 	require.NotNil(t, linker)
// }

// func TestGenerateLinkCard(t *testing.T) {
// 	linker, err := NewLinker(context.Background(), []byte(input))
// 	require.NoError(t, err)
// 	require.NotNil(t, linker)

// 	u, err := url.Parse("https://stream.place/iame.li")
// 	require.NoError(t, err)
// 	sp := "https://stream.place"
// 	ls := &streamplace.Livestream{
// 		CreatedAt: "2025-03-25T00:39:49.121Z",
// 		Post: &atproto.RepoStrongRef{
// 			Cid: "bafyreiczmyne5jd4lpax5ttyb5p2fbcageyt6fsthdpyymecokcsmyh4a4",
// 			Uri: "at://did:plc:2zmxikig2sj7gqaezl5gntae/app.bsky.feed.post/3ll5zuomua22x",
// 		},
// 		Title: "We're back up! Once again water in the firehose. Link cards if this stays stable",
// 		Url:   &sp,
// 	}
// 	lsv := &streamplace.Livestream_LivestreamView{
// 		Author: &bsky.ActorDefs_ProfileViewBasic{
// 			Handle: "iame.li",
// 			Did:    "did:plc:2zmxikig2sj7gqaezl5gntae",
// 		},
// 		Cid:       "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery",
// 		IndexedAt: "2025-03-25T01:16:14Z",
// 		Record:    &lexutil.LexiconTypeDecoder{Val: ls},
// 		Uri:       "at://did:plc:2zmxikig2sj7gqaezl5gntae/place.stream.livestream/3ll5zuop2k22x",
// 	}
// 	linkCard, err := linker.GenerateStreamerCard(context.Background(), u, lsv)
// 	require.NoError(t, err)
// 	require.Equal(t, outputStreamerCard, string(linkCard))
// }

// func TestGenerateDefaultCard(t *testing.T) {
// 	linker, err := NewLinker(context.Background(), []byte(input))
// 	require.NoError(t, err)
// 	require.NotNil(t, linker)

// 	u, err := url.Parse("https://stream.place/iame.li")
// 	require.NoError(t, err)
// 	linkCard, err := linker.GenerateDefaultCard(context.Background(), u)
// 	require.NoError(t, err)
// 	require.Equal(t, outputDefaultCard, string(linkCard))
// }

// const input string = `<!doctype html>
// <html lang="en">
//   <head>
//     <meta charset="utf-8" />
//     <meta httpEquiv="X-UA-Compatible" content="IE=edge" />
//     <meta
//       name="viewport"
//       content="width=device-width, initial-scale=1, shrink-to-fit=no"
//     />
//     <title>Streamplace</title>
//     <style id="expo-reset">
//       html,
//       body {
//         height: 100%;
//       }
//       body {
//         overflow: hidden;
//       }
//       #root {
//         display: flex;
//         height: 100%;
//         flex: 1;
//       }
//     </style>
//     <style>
//       html {
//         background-color: black;
//       }
//     </style>
//   <link rel="preload" href="/_expo/static/css/index-90f1b618e9e200dcb98a9b55d1941582.css" as="style"><link rel="stylesheet" href="/_expo/static/css/index-90f1b618e9e200dcb98a9b55d1941582.css"><link rel="shortcut icon" href="/favicon.ico" /></head>

//   <body>
//     <noscript> You need to enable JavaScript to run this app. </noscript>
//     <div id="root"></div>
//   <script src="/_expo/static/js/web/entrypoint-35bf1c89449fbda0007e7522d13b215f.js" defer></script>
// </body>
// </html>`

// const outputStreamerCard string = `<!DOCTYPE html><html lang="en"><head>
//     <meta charset="utf-8"/>
//     <meta httpequiv="X-UA-Compatible" content="IE=edge"/>
//     <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no"/>
//     <title>Streamplace</title>
//     <style id="expo-reset">
//       html,
//       body {
//         height: 100%;
//       }
//       body {
//         overflow: hidden;
//       }
//       #root {
//         display: flex;
//         height: 100%;
//         flex: 1;
//       }
//     </style>
//     <style>
//       html {
//         background-color: black;
//       }
//     </style>
//   <link rel="preload" href="/_expo/static/css/index-90f1b618e9e200dcb98a9b55d1941582.css" as="style"/><link rel="stylesheet" href="/_expo/static/css/index-90f1b618e9e200dcb98a9b55d1941582.css"/><link rel="shortcut icon" href="/favicon.ico"/><title>Stream.place</title><meta name="description" content="Stream.place is open-source livestreaming on the AT Protocol."/><meta property="og:url" content="https://stream.place/iame.li"/><meta property="og:type" content="website"/><meta property="og:title" content="Stream.place"/><meta property="og:description" content="Stream.place is open-source livestreaming on the AT Protocol."/><meta property="og:image" content="https://stream.place/linkbanner.png"/><meta name="twitter:card" content="summary_large_image"/><meta property="twitter:domain" content="stream.place"/><meta property="twitter:url" content="https://stream.place/iame.li"/><meta name="twitter:title" content="Stream.place"/><meta name="twitter:description" content="Stream.place is open-source livestreaming on the AT Protocol."/><meta name="twitter:image" content="https://stream.place/linkbanner.png"/></head>

//   <body>
//     <noscript> You need to enable JavaScript to run this app. </noscript>
//     <div id="root"></div>
//   <script src="/_expo/static/js/web/entrypoint-35bf1c89449fbda0007e7522d13b215f.js" defer=""></script>

// </body></html>`
// const outputDefaultCard string = ``
