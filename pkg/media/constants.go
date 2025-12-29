package media

var InterleaveBytes = uint64(0)
var InterleaveTime = uint64(0)

// BlurWarnings defines which content warnings should trigger thumbnail blurring
var BlurWarnings = map[string]bool{
	// C2PA format
	"cwarn:violence":  true,
	"cwarn:death":     true,
	"cwarn:suffering": true,
	"cwarn:nudity":    true,
	"cwarn:sexuality": true,
	// Lexicon format
	"place.stream.metadata.contentWarnings#violence":  true,
	"place.stream.metadata.contentWarnings#death":     true,
	"place.stream.metadata.contentWarnings#suffering": true,
	"place.stream.metadata.contentWarnings#nudity":    true,
	"place.stream.metadata.contentWarnings#sexuality": true,
}
