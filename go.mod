module aquareum.tv/aquareum

go 1.22.4

replace github.com/livepeer/lpms => github.com/aquareum-tv/lpms v0.0.0-20240828210246-5ac9b407751e

replace github.com/ThalesGroup/crypto11 => github.com/aquareum-tv/crypto11 v0.0.0-20240821184406-43336abc768f

require (
	firebase.google.com/go/v4 v4.14.1
	git.aquareum.tv/aquareum-tv/c2pa-go v0.0.0-20240913223408-68f9878542d4
	github.com/NYTimes/gziphandler v1.1.1
	github.com/ThalesGroup/crypto11 v0.0.0-00010101000000-000000000000
	github.com/decred/dcrd/dcrec/secp256k1 v1.0.4
	github.com/dunglas/httpsfv v1.0.2
	github.com/ethereum/go-ethereum v1.14.7
	github.com/go-git/go-git/v5 v5.12.0
	github.com/go-gst/go-glib v1.3.0
	github.com/go-gst/go-gst v1.3.0
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/golang/glog v1.2.0
	github.com/google/uuid v1.6.0
	github.com/johncgriffin/overflow v0.0.0-20211019200055-46fa312c352c
	github.com/julienschmidt/httprouter v1.3.0
	github.com/livepeer/lpms v0.0.0-20240812093642-b5181eb92cb2
	github.com/lmittmann/tint v1.0.4
	github.com/orandin/slog-gorm v1.3.2
	github.com/peterbourgon/ff/v3 v3.3.1
	github.com/piprate/json-gold v0.5.0
	github.com/rs/cors v1.7.0
	github.com/samber/slog-http v1.4.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/stretchr/testify v1.9.0
	gitlab.com/gitlab-org/release-cli v0.18.0
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/image v0.22.0
	golang.org/x/net v0.29.0
	golang.org/x/sync v0.9.0
	golang.org/x/term v0.24.0
	google.golang.org/api v0.189.0
	gorm.io/datatypes v1.2.4
	gorm.io/driver/sqlite v1.5.5
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.20
	golang.org/x/sys v0.25.0 // indirect
	gorm.io/gorm v1.25.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	cloud.google.com/go v0.115.0 // indirect
	cloud.google.com/go/auth v0.7.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.3 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/firestore v1.15.0 // indirect
	cloud.google.com/go/iam v1.1.10 // indirect
	cloud.google.com/go/longrunning v0.5.9 // indirect
	cloud.google.com/go/storage v1.41.0 // indirect
	dario.cat/mergo v1.0.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/MicahParks/keyfunc v1.9.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/bits-and-blooms/bitset v1.10.0 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/consensys/gnark-crypto v0.12.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/crate-crypto/go-kzg-4844 v1.0.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v2 v2.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/ethereum/c-kzg-4844 v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.5 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.4.0 // indirect
	github.com/holiman/uint256 v1.3.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jstemmer/go-junit-report v1.0.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/livepeer/m3u8 v0.11.1 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mitchellh/gox v1.0.1 // indirect
	github.com/mitchellh/iochan v1.0.0 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.2.2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/supranational/blst v0.3.11 // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/urfave/cli/v2 v2.27.5 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/appengine/v2 v2.0.2 // indirect
	google.golang.org/genproto v0.0.0-20240722135656-d784300faade // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240722135656-d784300faade // indirect
	google.golang.org/grpc v1.64.1 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gorm.io/driver/mysql v1.5.6 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)
