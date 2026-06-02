package constants

import "fmt"

var PLACE_STREAM_KEY = "place.stream.key"                                     //nolint:all
var PLACE_STREAM_LIVESTREAM = "place.stream.livestream"                       //nolint:all
var PLACE_STREAM_CHAT_MESSAGE = "place.stream.chat.message"                   //nolint:all
var PLACE_STREAM_CHAT_PROFILE = "place.stream.chat.profile"                   //nolint:all
var PLACE_STREAM_SERVER_SETTINGS = "place.stream.server.settings"             //nolint:all
var PLACE_STREAM_LIVE_TELEPORT = "place.stream.live.teleport"                 //nolint:all
var PLACE_STREAM_MODERATION_PERMISSION = "place.stream.moderation.permission" //nolint:all
var STREAMPLACE_SIGNING_KEY = "signingKey"                                    //nolint:all
var APP_BSKY_GRAPH_FOLLOW = "app.bsky.graph.follow"                           //nolint:all
var APP_BSKY_FEED_POST = "app.bsky.feed.post"                                 //nolint:all
var APP_BSKY_GRAPH_BLOCK = "app.bsky.graph.block"                             //nolint:all
var APP_BSKY_ACTOR_PROFILE = "app.bsky.actor.profile"                         //nolint:all
var PLACE_STREAM_CHAT_GATE = "place.stream.chat.gate"                         //nolint:all
var PLACE_STREAM_CHAT_PINNED_RECORD = "place.stream.chat.pinnedRecord"        //nolint:all
var PLACE_STREAM_DEFAULT_METADATA = "place.stream.metadata.configuration"     //nolint:all
var PLACE_STREAM_LIVE_RECOMMENDATIONS = "place.stream.live.recommendations"   //nolint:all
var PLACE_STREAM_LIVE_VIEWERCOUNT = "place.stream.live.viewerCount"           //nolint:all                      //nolint:all
var PLACE_STREAM_BADGE_DEF = "place.stream.badge.def"                         //nolint:all
var PLACE_STREAM_BADGE_ISSUANCE = "place.stream.badge.issuance"               //nolint:all
var PLACE_STREAM_VIDEO = "place.stream.video"                                 //nolint:all
var PLACE_STREAM_MEDIA_TRACK = "place.stream.media.track"                     //nolint:all
var PLACE_STREAM_MEDIA_ORIGIN = "place.stream.media.origin"                   //nolint:all
var PLACE_STREAM_MEDIA_VIEW_COUNT = "place.stream.media.viewCount"            //nolint:all
var PLACE_STREAM_BETA_INVITE = "place.stream.beta.invite"                     //nolint:all
var PLACE_STREAM_BETA_REQUEST = "place.stream.beta.request"                   //nolint:all
var PLACE_STREAM_VOD_COMMENT = "place.stream.vod.comment"                     //nolint:all
var PLACE_STREAM_LIKE = "place.stream.like"                                   //nolint:all
var PLACE_STREAM_VOD_GATE = "place.stream.vod.gate"                           //nolint:all

// Streamplace badge types
const (
	BadgeTypeMod      = "place.stream.badge.defs#mod"
	BadgeTypeBot      = "place.stream.badge.defs#bot"
	BadgeTypeStreamer = "place.stream.badge.defs#streamer"
	BadgeTypeVIP      = "place.stream.badge.defs#vip"
	BadgeTypeEvent    = "place.stream.badge.defs#event"
)

// GlobalBadgeIssuers is the list of DIDs authorized to issue globally-valid badges
// (i.e. badges that appear in any streamer's chat once the recipient selects them).
// Add the streamplace node DID here for event/contest badges.
// We may want to limit this even further in the future to specific at URIs to avoid abuse
// but for now we'll trust these users
var GlobalBadgeIssuers = []string{"did:plc:2zmxikig2sj7gqaezl5gntae", "did:plc:k644h4rq5bjfzcetgsa6tuby", "did:plc:rbvrr34edl5ddpuwcubjiost"} //nolint:all

const SelfLabelBot = "bot" //nolint:all

const DID_KEY_PREFIX = "did:key" //nolint:all
const ADDRESS_KEY_PREFIX = "0x"  //nolint:all

// Streamplace metadata license values
const (
	LicenseCC0_1_0           = "place.stream.metadata.contentRights#cc0_1__0"
	LicenseCCBy_4_0          = "place.stream.metadata.contentRights#cc-by_4__0"
	LicenseCCBySA_4_0        = "place.stream.metadata.contentRights#cc-by-sa_4__0"
	LicenseCCByNC_4_0        = "place.stream.metadata.contentRights#cc-by-nc_4__0"
	LicenseCCByNCSA_4_0      = "place.stream.metadata.contentRights#cc-by-nc-sa_4__0"
	LicenseCCByND_4_0        = "place.stream.metadata.contentRights#cc-by-nd_4__0"
	LicenseCCByNCND_4_0      = "place.stream.metadata.contentRights#cc-by-nc-nd_4__0"
	LicenseAllRightsReserved = "place.stream.metadata.contentRights#all-rights-reserved"
)

// License URLs for C2PA manifests
const (
	LicenseURLCC0_1_0      = "http://creativecommons.org/publicdomain/zero/1.0/"
	LicenseURLCCBy_4_0     = "http://creativecommons.org/licenses/by/4.0/"
	LicenseURLCCBySA_4_0   = "http://creativecommons.org/licenses/by-sa/4.0/"
	LicenseURLCCByNC_4_0   = "http://creativecommons.org/licenses/by-nc/4.0/"
	LicenseURLCCByNCSA_4_0 = "http://creativecommons.org/licenses/by-nc-sa/4.0/"
	LicenseURLCCByND_4_0   = "http://creativecommons.org/licenses/by-nd/4.0/"
	LicenseURLCCByNCND_4_0 = "http://creativecommons.org/licenses/by-nc-nd/4.0/"
)

// Streamplace metadata warning labels
const (
	WarningDeath           = "place.stream.metadata.contentWarnings#death"
	WarningDrugUse         = "place.stream.metadata.contentWarnings#drugUse"
	WarningFantasyViolence = "place.stream.metadata.contentWarnings#fantasyViolence"
	WarningFlashingLights  = "place.stream.metadata.contentWarnings#flashingLights"
	WarningLanguage        = "place.stream.metadata.contentWarnings#language"
	WarningNudity          = "place.stream.metadata.contentWarnings#nudity"
	WarningPII             = "place.stream.metadata.contentWarnings#PII"
	WarningSexuality       = "place.stream.metadata.contentWarnings#sexuality"
	WarningSuffering       = "place.stream.metadata.contentWarnings#suffering"
	WarningViolence        = "place.stream.metadata.contentWarnings#violence"
)

// Content warning C2PA codes for manifests
const (
	WarningC2PADeath           = "cwarn:death"
	WarningC2PADrugUse         = "cwarn:drugUse"
	WarningC2PAFantasyViolence = "cwarn:fantasyViolence"
	WarningC2PAFlashingLights  = "cwarn:flashingLights"
	WarningC2PALanguage        = "cwarn:language"
	WarningC2PANudity          = "cwarn:nudity"
	WarningC2PAPII             = "cwarn:PII"
	WarningC2PASexuality       = "cwarn:sexuality"
	WarningC2PASuffering       = "cwarn:suffering"
	WarningC2PAViolence        = "cwarn:violence"
)

const BlueskyProfileGoliveKey = "place.stream.live.golive"

const QueueMaxSizeBuffers = uint(0)
const QueueMaxSizeTime = uint(0)
const QueueMaxSizeBytes = uint(50000000)

var Queue2Big = fmt.Sprintf("queue2 max-size-buffers=%d max-size-bytes=%d max-size-time=%d", QueueMaxSizeBuffers, QueueMaxSizeBytes, QueueMaxSizeTime)
