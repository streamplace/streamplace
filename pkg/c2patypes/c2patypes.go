// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    c2Patypes, err := UnmarshalC2Patypes(bytes)
//    bytes, err = c2Patypes.Marshal()

package c2patypes

import "bytes"
import "errors"

import "encoding/json"

func UnmarshalC2Patypes(data []byte) (C2Patypes, error) {
	var r C2Patypes
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *C2Patypes) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type C2Patypes struct {
	Builder            Builder            `json:"builder"`
	ManifestDefinition ManifestDefinition `json:"manifest_definition"`
	Reader             Reader             `json:"reader"`
}

// Use a Builder to add a signed manifest to an asset.
//
// # Example: Building and signing a manifest
//
// ``` use c2pa::Result; use std::path::PathBuf;
//
// use c2pa::{create_signer, Builder, SigningAlg}; use serde::Serialize; use
// serde_json::json; use tempfile::tempdir;
//
// #[derive(Serialize)] struct Test { my_tag: usize, }
//
// # fn main() -> Result<()> { #[cfg(feature = "file_io")] { let manifest_json = json!({
// "claim_generator_info": [ { "name": "c2pa_test", "version": "1.0.0" } ], "title":
// "Test_Manifest" }).to_string();
//
// let mut builder = Builder::from_json(&manifest_json)?;
// builder.add_assertion("org.contentauth.test", &Test { my_tag: 42 })?;
//
// let source = PathBuf::from("tests/fixtures/C.jpg"); let dir = tempdir()?; let dest =
// dir.path().join("test_file.jpg");
//
// // Create a ps256 signer using certs and key files. TO DO: Update example. let
// signcert_path = "tests/fixtures/certs/ps256.pub"; let pkey_path =
// "tests/fixtures/certs/ps256.pem"; let signer = create_signer::from_files(signcert_path,
// pkey_path, SigningAlg::Ps256, None)?;
//
// // embed a manifest using the signer builder.sign_file( signer.as_ref(), &source,
// &dest)?; } # Ok(()) # } ```
type Builder struct {
	// A list of assertions
	Assertions []AssertionDefinition `json:"assertions,omitempty"`
	// Base path to search for resources.
	BasePath *string `json:"base_path"`
	// The type of builder being used.
	BuilderFlow *BuilderFlowUnion `json:"builder_flow"`
	// Claim Generator Info is always required with at least one entry
	ClaimGeneratorInfo []ClaimGeneratorInfo `json:"claim_generator_info,omitempty"`
	// The version of the claim.  Defaults to 1.
	ClaimVersion *int64 `json:"claim_version"`
	// The format of the source file as a MIME type.
	Format *string `json:"format,omitempty"`
	// A List of ingredients
	Ingredients []Ingredient `json:"ingredients,omitempty"`
	// Instance ID from `xmpMM:InstanceID` in XMP metadata.
	InstanceID *string `json:"instance_id,omitempty"`
	// Allows you to pre-define the manifest label, which must be unique. Not intended for
	// general use.  If not set, it will be assigned automatically.
	Label *string `json:"label"`
	// Optional manifest metadata. This will be deprecated in the future; not recommended to use.
	Metadata []AssertionMetadata `json:"metadata"`
	// If true, the manifest store will not be embedded in the asset on sign
	NoEmbed bool `json:"no_embed"`
	// A list of redactions - URIs to redacted assertions.
	Redactions []string `json:"redactions"`
	// Optional remote URL for the manifest
	RemoteURL *string `json:"remote_url"`
	// An optional ResourceRef to a thumbnail image that represents the asset that was signed.
	// Must be available when the manifest is signed.
	Thumbnail *IngredientResourceRef `json:"thumbnail"`
	// A human-readable title, generally source filename.
	Title *string `json:"title"`
	// Optional prefix added to the generated Manifest Label This is typically a reverse domain
	// name.
	Vendor *string `json:"vendor"`
}

// Defines an assertion that consists of a label that can be either a C2PA-defined assertion
// label or a custom label in reverse domain format.
type AssertionDefinition struct {
	Data  interface{} `json:"data"`
	Label string      `json:"label"`
}

type BuilderFlowClass struct {
	Create DigitalSourceType `json:"Create"`
}

// Description of the claim generator, or the software used in generating the claim.
//
// This structure is also used for actions softwareAgent
type ClaimGeneratorInfo struct {
	// hashed URI to the icon (either embedded or remote)
	Icon *ResourceRef `json:"icon"`
	// A human readable string naming the claim_generator
	Name string `json:"name"`
	// A human readable string of the OS the claim generator is running on
	OperatingSystem *string `json:"operating_system"`
	// A human readable string of the product's version
	Version *string `json:"version"`
}

// A reference to a resource to be used in JSON serialization.
//
// The underlying data can be read as a stream via
// [`Reader::resource_to_stream`][crate::Reader::resource_to_stream].
//
// A `HashedUri` provides a reference to content available within the same manifest store.
//
// This is described in [§8.3, URI References], of the C2PA Technical Specification.
//
// [§8.3, URI References]:
// https://c2pa.org/specifications/specifications/2.1/specs/C2PA_Specification.html#_uri_references
type ResourceRef struct {
	// The algorithm used to hash the resource (if applicable).
	//
	// A string identifying the cryptographic hash algorithm used to compute the hash
	Alg *string `json:"alg"`
	// More detailed data types as defined in the C2PA spec.
	DataTypes []AssetType `json:"data_types"`
	// The mime type of the referenced resource.
	Format *string `json:"format,omitempty"`
	// The hash of the resource (if applicable).
	//
	// Byte string containing the hash value
	Hash *BasePath `json:"hash"`
	// A URI that identifies the resource as referenced from the manifest.
	//
	// This may be a JUMBF URI, a file path, a URL or any other string. Relative JUMBF URIs will
	// be resolved with the manifest label. Relative file paths will be resolved with the base
	// path if provided.
	Identifier *string `json:"identifier,omitempty"`
	// JUMBF URI reference
	URL *string `json:"url,omitempty"`
}

type AssetType struct {
	Type    string  `json:"type"`
	Version *string `json:"version"`
}

// An `Ingredient` is any external asset that has been used in the creation of an asset.
type Ingredient struct {
	// The active manifest label (if one exists).
	//
	// If this ingredient has a [`ManifestStore`], this will hold the label of the active
	// [`Manifest`].
	//
	// [`Manifest`]: crate::Manifest [`ManifestStore`]: crate::ManifestStore
	ActiveManifest *string `json:"active_manifest"`
	// A reference to the actual data of the ingredient.
	Data *IngredientResourceRef `json:"data"`
	// Additional information about the data's type to the ingredient V2 structure.
	DataTypes []AssetType `json:"data_types"`
	// Additional description of the ingredient.
	Description *string `json:"description"`
	// Document ID from `xmpMM:DocumentID` in XMP metadata.
	DocumentID *string `json:"document_id"`
	// The format of the source file as a MIME type.
	Format *string `json:"format"`
	// An optional hash of the asset to prevent duplicates.
	Hash *string `json:"hash"`
	// URI to an informational page about the ingredient or its data.
	InformationalURI *string `json:"informational_URI"`
	// Instance ID from `xmpMM:InstanceID` in XMP metadata.
	InstanceID *string `json:"instance_id"`
	// The ingredient's label as assigned in the manifest.
	Label *string `json:"label"`
	// A [`ManifestStore`] from the source asset extracted as a binary C2PA blob.
	//
	// [`ManifestStore`]: crate::ManifestStore
	ManifestData *IngredientResourceRef `json:"manifest_data"`
	// Any additional [`Metadata`] as defined in the C2PA spec.
	//
	// [`Metadata`]: crate::Metadata
	Metadata *AssertionMetadata `json:"metadata"`
	// URI from `dcterms:provenance` in XMP metadata.
	Provenance *string `json:"provenance"`
	// Set to `ParentOf` if this is the parent ingredient.
	//
	// There can only be one parent ingredient in the ingredients.
	Relationship *Relationship  `json:"relationship,omitempty"`
	Resources    *ResourceStore `json:"resources,omitempty"`
	// A thumbnail image capturing the visual state at the time of import.
	//
	// A tuple of thumbnail MIME format (for example `image/jpeg`) and binary bits of the image.
	Thumbnail *IngredientResourceRef `json:"thumbnail"`
	// A human-readable title, generally source filename.
	Title *string `json:"title"`
	// Validation results (Ingredient.V3)
	ValidationResults *ValidationResults `json:"validation_results"`
	// Validation status (Ingredient v1 & v2)
	ValidationStatus []ValidationStatus `json:"validation_status"`
}

// A reference to a resource to be used in JSON serialization.
//
// The underlying data can be read as a stream via
// [`Reader::resource_to_stream`][crate::Reader::resource_to_stream].
type IngredientResourceRef struct {
	// The algorithm used to hash the resource (if applicable).
	Alg *string `json:"alg"`
	// More detailed data types as defined in the C2PA spec.
	DataTypes []AssetType `json:"data_types"`
	// The mime type of the referenced resource.
	Format string `json:"format"`
	// The hash of the resource (if applicable).
	Hash *string `json:"hash"`
	// A URI that identifies the resource as referenced from the manifest.
	//
	// This may be a JUMBF URI, a file path, a URL or any other string. Relative JUMBF URIs will
	// be resolved with the manifest label. Relative file paths will be resolved with the base
	// path if provided.
	Identifier string `json:"identifier"`
}

// A region of interest within an asset describing the change.
//
// This struct can be used from [`Action::changes`][crate::assertions::Action::changes] or
// [`AssertionMetadata::region_of_interest`][crate::assertions::AssertionMetadata::region_of_interest].
type RegionOfInterest struct {
	// A free-text string.
	Description *string `json:"description"`
	// A free-text string representing a machine-readable, unique to this assertion, identifier
	// for the region.
	Identifier *string `json:"identifier"`
	// Additional information about the asset.
	Metadata *AssertionMetadata `json:"metadata"`
	// A free-text string representing a human-readable name for the region which might be used
	// in a user interface.
	Name *string `json:"name"`
	// A range describing the region of interest for the specific asset.
	Region []Range `json:"region"`
	// A value from our controlled vocabulary or an entity-specific value (e.g.,
	// com.litware.coolArea) that represents the role of a region among other regions.
	Role *Role `json:"role"`
	// A value from a controlled vocabulary such as
	// <https://cv.iptc.org/newscodes/imageregiontype/> or an entity-specific value (e.g.,
	// com.litware.newType) that represents the type of thing(s) depicted by a region.
	//
	// Note this field serializes/deserializes into the name `type`.
	Type *string `json:"type"`
}

// The AssertionMetadata structure can be used as part of other assertions or on its own to
// reference others
type AssertionMetadata struct {
	DataSource       *DataSource       `json:"dataSource"`
	DateTime         *string           `json:"dateTime"`
	Reference        *HashedURI        `json:"reference"`
	RegionOfInterest *RegionOfInterest `json:"regionOfInterest"`
	ReviewRatings    []ReviewRating    `json:"reviewRatings"`
}

// A spatial, temporal, frame, or textual range describing the region of interest.
type Range struct {
	// A frame range.
	Frame *Frame `json:"frame"`
	// A item identifier.
	Item *Item `json:"item"`
	// A spatial range.
	Shape *Shape `json:"shape"`
	// A textual range.
	Text *Text `json:"text"`
	// A temporal range.
	Time *Time `json:"time"`
	// The type of range of interest.
	Type RangeType `json:"type"`
}

// A frame range representing starting and ending frames or pages.
//
// If both `start` and `end` are missing, the frame will span the entire asset.
type Frame struct {
	// The end of the frame inclusive or the end of the asset if not present.
	End *int64 `json:"end"`
	// The start of the frame or the end of the asset if not present.
	//
	// The first frame/page starts at 0.
	Start *int64 `json:"start"`
}

// Description of the boundaries of an identified range.
type Item struct {
	// The container-specific term used to identify items, such as "track_id" for MP4 or
	// "item_ID" for HEIF.
	Identifier string `json:"identifier"`
	// The value of the identifier, e.g. a value of "2" for an identifier of "track_id" would
	// imply track 2 of the asset.
	Value string `json:"value"`
}

// A spatial range representing rectangle, circle, or a polygon.
type Shape struct {
	// The height of a rectnagle.
	//
	// This field can be ignored for circles and polygons.
	Height *float64 `json:"height"`
	// If the range is inside the shape.
	//
	// The default value is true.
	Inside *bool `json:"inside"`
	// THe origin of the coordinate in the shape.
	Origin Coordinate `json:"origin"`
	// The type of shape.
	Type ShapeType `json:"type"`
	// The type of unit for the shape range.
	Unit UnitType `json:"unit"`
	// The vertices of the polygon.
	//
	// This field can be ignored for rectangles and circles.
	Vertices []Coordinate `json:"vertices"`
	// The width for rectangles or diameter for circles.
	//
	// This field can be ignored for polygons.
	Width *float64 `json:"width"`
}

// THe origin of the coordinate in the shape.
//
// An x, y coordinate used for specifying vertices in polygons.
type Coordinate struct {
	// The coordinate along the x-axis.
	X float64 `json:"x"`
	// The coordinate along the y-axis.
	Y float64 `json:"y"`
}

// A textual range representing multiple (possibly discontinuous) ranges of text.
type Text struct {
	// The ranges of text to select.
	Selectors []TextSelectorRange `json:"selectors"`
}

// One or two [`TextSelector`][TextSelector] identifiying the range to select.
type TextSelectorRange struct {
	// The end of the text range.
	End *TextSelector `json:"end"`
	// The start (or entire) text range.
	Selector TextSelector `json:"selector"`
}

// Selects a range of text via a fragment identifier.
//
// This is modeled after the W3C Web Annotation selector model.
//
// The start (or entire) text range.
type TextSelector struct {
	// The end character offset or the end of the fragment if not present.
	End *int64 `json:"end"`
	// Fragment identifier as per RFC3023 (XML) or ISO 32000-2 (PDF), Annex O.
	Fragment string `json:"fragment"`
	// The start character offset or the start of the fragment if not present.
	Start *int64 `json:"start"`
}

// A temporal range representing a starting time to an ending time.
type Time struct {
	// The end time or the end of the asset if not present.
	End *string `json:"end"`
	// The start time or the start of the asset if not present.
	Start *string `json:"start"`
	// The type of time.
	Type *TimeType `json:"type,omitempty"`
}

// A description of the source for assertion data
type DataSource struct {
	// A list of [`Actor`]s associated with this source.
	Actors []Actor `json:"actors"`
	// A human-readable string giving details about the source of the assertion data.
	Details *string `json:"details"`
	// A value from among the enumerated list indicating the source of the assertion.
	Type string `json:"type"`
}

// Identifies a person responsible for an action.
type Actor struct {
	// List of references to W3C Verifiable Credentials.
	Credentials []HashedURI `json:"credentials"`
	// An identifier for a human actor, used when the "type" is `humanEntry.identified`.
	Identifier *string `json:"identifier"`
}

// A `HashedUri` provides a reference to content available within the same manifest store.
//
// This is described in [§8.3, URI References], of the C2PA Technical Specification.
//
// [§8.3, URI References]:
// https://c2pa.org/specifications/specifications/2.1/specs/C2PA_Specification.html#_uri_references
type HashedURI struct {
	// A string identifying the cryptographic hash algorithm used to compute the hash
	Alg *string `json:"alg"`
	// Byte string containing the hash value
	Hash []int64 `json:"hash"`
	// JUMBF URI reference
	URL string `json:"url"`
}

// A rating on an Assertion.
//
// See
// <https://c2pa.org/specifications/specifications/2.2/specs/C2PA_Specification.html#_review_ratings>.
type ReviewRating struct {
	Code        *string `json:"code"`
	Explanation string  `json:"explanation"`
	Value       int64   `json:"value"`
}

// Resource store to contain binary objects referenced from JSON serializable structures
//
// container for binary assets (like thumbnails)
type ResourceStore struct {
	BasePath  *string            `json:"base_path"`
	Label     *string            `json:"label"`
	Resources map[string][]int64 `json:"resources"`
}

// A map of validation results for a manifest store.
//
// The map contains the validation results for the active manifest and any ingredient
// deltas. It is normal for there to be many
type ValidationResults struct {
	ActiveManifest   *StatusCodes                      `json:"activeManifest"`
	IngredientDeltas []IngredientDeltaValidationResult `json:"ingredientDeltas"`
}

// Contains a set of success, informational, and failure validation status codes.
//
// Validation results for the ingredient's active manifest
type StatusCodes struct {
	Failure       []ValidationStatus `json:"failure"`
	Informational []ValidationStatus `json:"informational"`
	Success       []ValidationStatus `json:"success"`
}

// A `ValidationStatus` struct describes the validation status of a specific part of a
// manifest.
//
// See
// <https://c2pa.org/specifications/specifications/2.2/specs/C2PA_Specification.html#_existing_manifests>.
type ValidationStatus struct {
	Code        string  `json:"code"`
	Explanation *string `json:"explanation"`
	Success     *bool   `json:"success"`
	URL         *string `json:"url"`
}

// Represents any changes or deltas between the current and previous validation results for
// an ingredient's manifest.
type IngredientDeltaValidationResult struct {
	// JUMBF URI reference to the ingredient assertion
	IngredientAssertionURI string `json:"ingredientAssertionURI"`
	// Validation results for the ingredient's active manifest
	ValidationDeltas StatusCodes `json:"validationDeltas"`
}

// Use a ManifestDefinition to define a manifest and to build a `ManifestStore`. A manifest
// is a collection of ingredients and assertions used to define a claim that can be signed
// and embedded into a file.
type ManifestDefinition struct {
	// A list of assertions
	Assertions []AssertionDefinition `json:"assertions,omitempty"`
	// Claim Generator Info is always required with at least one entry
	ClaimGeneratorInfo []ClaimGeneratorInfo `json:"claim_generator_info,omitempty"`
	// The version of the claim.  Defaults to 1.
	ClaimVersion *int64 `json:"claim_version"`
	// The format of the source file as a MIME type.
	Format *string `json:"format,omitempty"`
	// A List of ingredients
	Ingredients []Ingredient `json:"ingredients,omitempty"`
	// Instance ID from `xmpMM:InstanceID` in XMP metadata.
	InstanceID *string `json:"instance_id,omitempty"`
	// Allows you to pre-define the manifest label, which must be unique. Not intended for
	// general use.  If not set, it will be assigned automatically.
	Label *string `json:"label"`
	// Optional manifest metadata. This will be deprecated in the future; not recommended to use.
	Metadata []AssertionMetadata `json:"metadata"`
	// A list of redactions - URIs to redacted assertions.
	Redactions []string `json:"redactions"`
	// An optional ResourceRef to a thumbnail image that represents the asset that was signed.
	// Must be available when the manifest is signed.
	Thumbnail *IngredientResourceRef `json:"thumbnail"`
	// A human-readable title, generally source filename.
	Title *string `json:"title"`
	// Optional prefix added to the generated Manifest Label This is typically a reverse domain
	// name.
	Vendor *string `json:"vendor"`
}

// Use a Reader to read and validate a manifest store.
type Reader struct {
	// A label for the active (most recent) manifest in the store
	ActiveManifest *string `json:"active_manifest"`
	// A HashMap of Manifests
	Manifests map[string]Manifest `json:"manifests"`
	// ValidationStatus generated when loading the ManifestStore from an asset
	ValidationResults *ValidationResults `json:"validation_results"`
	// The validation state of the manifest store
	ValidationState *ValidationState `json:"validation_state"`
	// ValidationStatus generated when loading the ManifestStore from an asset
	ValidationStatus []ValidationStatus `json:"validation_status"`
}

// A Manifest represents all the information in a c2pa manifest
type Manifest struct {
	// A list of assertions
	Assertions []ManifestAssertion `json:"assertions,omitempty"`
	// A User Agent formatted string identifying the software/hardware/system produced this
	// claim Spaces are not allowed in names, versions can be specified with product/1.0 syntax.
	ClaimGenerator *string `json:"claim_generator"`
	// A list of claim generator info data identifying the software/hardware/system produced
	// this claim.
	ClaimGeneratorInfo []ClaimGeneratorInfo `json:"claim_generator_info"`
	// A List of verified credentials
	Credentials []interface{} `json:"credentials"`
	// The format of the source file as a MIME type.
	Format *string `json:"format"`
	// A List of ingredients
	Ingredients []Ingredient `json:"ingredients,omitempty"`
	// Instance ID from `xmpMM:InstanceID` in XMP metadata.
	InstanceID *string `json:"instance_id,omitempty"`
	Label      *string `json:"label"`
	// A list of user metadata for this claim.
	Metadata []AssertionMetadata `json:"metadata"`
	// A list of redactions - URIs to a redacted assertions
	Redactions []string `json:"redactions"`
	// container for binary assets (like thumbnails)
	Resources *ResourceStore `json:"resources,omitempty"`
	// Signature data (only used for reporting)
	SignatureInfo *SignatureInfo         `json:"signature_info"`
	Thumbnail     *IngredientResourceRef `json:"thumbnail"`
	// A human-readable title, generally source filename.
	Title *string `json:"title"`
	// Optional prefix added to the generated Manifest label. This is typically an internet
	// domain name for the vendor (i.e. `adobe`).
	Vendor *string `json:"vendor"`
}

// A labeled container for an Assertion value in a Manifest
type ManifestAssertion struct {
	// The data of the assertion as Value
	Data interface{} `json:"data"`
	// There can be more than one assertion for any label
	Instance *int64 `json:"instance"`
	// The [ManifestAssertionKind] for this assertion (as stored in c2pa content)
	Kind *ManifestAssertionKind `json:"kind"`
	// An assertion label in reverse domain format
	Label string `json:"label"`
}

// Holds information about a signature
type SignatureInfo struct {
	// Human-readable issuing authority for this signature.
	Alg *SigningAlg `json:"alg"`
	// The serial number of the certificate.
	CERTSerialNumber *string `json:"cert_serial_number"`
	// Human-readable issuing authority for this signature.
	Issuer *string `json:"issuer"`
	// Revocation status of the certificate.
	RevocationStatus *bool `json:"revocation_status"`
	// The time the signature was created.
	Time *string `json:"time"`
}

// Media whose digital content is effectively empty, such as a blank canvas or zero-length
// video.
//
// Data that is the result of algorithmically using a model derived from sampled content and
// data. Differs from
// <http://cv.iptc.org/newscodes/digitalsourcetype/>trainedAlgorithmicMedia in that the
// result isn’t a media type (e.g., image or video) but is a data format (e.g., CSV, pickle).
type DigitalSourceType string

const (
	Empty                  DigitalSourceType = "Empty"
	TrainedAlgorithmicData DigitalSourceType = "TrainedAlgorithmicData"
)

type BuilderFlowEnum string

const (
	Open   BuilderFlowEnum = "Open"
	Update BuilderFlowEnum = "Update"
)

// The type of shape.
//
// The type of shape for the range.
//
// A rectangle.
//
// A circle.
//
// A polygon.
type ShapeType string

const (
	Circle    ShapeType = "circle"
	Polygon   ShapeType = "polygon"
	Rectangle ShapeType = "rectangle"
)

// The type of unit for the shape range.
//
// The type of unit for the range.
//
// Use pixels.
//
// Use percentage.
type UnitType string

const (
	Percent UnitType = "percent"
	Pixel   UnitType = "pixel"
)

// The type of time.
//
// Times are described using Normal Play Time (npt) as described in RFC 2326.
type TimeType string

const (
	Npt TimeType = "npt"
)

// The type of range of interest.
//
// The type of range for the region of interest.
//
// A spatial range, see [`Shape`] for more details.
//
// A temporal range, see [`Time`] for more details.
//
// A spatial range, see [`Frame`] for more details.
//
// A textual range, see [`Text`] for more details.
//
// A range identified by a specific identifier and value, see [`Item`] for more details.
type RangeType string

const (
	Identified     RangeType = "identified"
	RangeTypeFrame RangeType = "frame"
	Spatial        RangeType = "spatial"
	Temporal       RangeType = "temporal"
	Textual        RangeType = "textual"
)

// Arbitrary area worth identifying.
//
// This area is all that is left after a crop action.
//
// This area has had edits applied to it.
//
// The area where an ingredient was placed/added.
//
// Something in this area was redacted.
//
// Area specific to a subject (human or not).
//
// A range of information was removed/deleted.
//
// Styling was applied to this area.
//
// Invisible watermarking was applied to this area for the purpose of soft binding.
type Role string

const (
	C2PaAreaOfInterest Role = "c2pa.areaOfInterest"
	C2PaCropped        Role = "c2pa.cropped"
	C2PaDeleted        Role = "c2pa.deleted"
	C2PaEdited         Role = "c2pa.edited"
	C2PaPlaced         Role = "c2pa.placed"
	C2PaRedacted       Role = "c2pa.redacted"
	C2PaStyled         Role = "c2pa.styled"
	C2PaSubjectArea    Role = "c2pa.subjectArea"
	C2PaWatermarked    Role = "c2pa.watermarked"
)

// Set to `ParentOf` if this is the parent ingredient.
//
// There can only be one parent ingredient in the ingredients.
//
// The relationship of the ingredient to the current asset.
//
// The current asset is derived from this ingredient.
//
// The current asset is a part of this ingredient.
//
// The ingredient was used as an input to a computational process to create or modify the
// asset.
type Relationship string

const (
	ComponentOf Relationship = "componentOf"
	InputTo     Relationship = "inputTo"
	ParentOf    Relationship = "parentOf"
)

// Assertions in C2PA can be stored in several formats
type ManifestAssertionKind string

const (
	Binary ManifestAssertionKind = "Binary"
	Cbor   ManifestAssertionKind = "Cbor"
	JSON   ManifestAssertionKind = "Json"
	URI    ManifestAssertionKind = "Uri"
)

// ECDSA with SHA-256
//
// # ECDSA with SHA-256 on secp256k1 curve
//
// # ECDSA with SHA-384
//
// # ECDSA with SHA-512
//
// # RSASSA-PSS using SHA-256 and MGF1 with SHA-256
//
// # RSASSA-PSS using SHA-384 and MGF1 with SHA-384
//
// # RSASSA-PSS using SHA-512 and MGF1 with SHA-512
//
// Edwards-Curve DSA (Ed25519 instance only)
type SigningAlg string

const (
	Ed25519 SigningAlg = "Ed25519"
	Es256   SigningAlg = "Es256"
	Es256K  SigningAlg = "Es256K"
	Es384   SigningAlg = "Es384"
	Es512   SigningAlg = "Es512"
	Ps256   SigningAlg = "Ps256"
	Ps384   SigningAlg = "Ps384"
	Ps512   SigningAlg = "Ps512"
)

// Errors were found in the manifest store.
//
// No errors were found in validation, but the active signature is not trusted.
//
// The manifest store is valid and the active signature is trusted.
type ValidationState string

const (
	Invalid ValidationState = "Invalid"
	Trusted ValidationState = "Trusted"
	Valid   ValidationState = "Valid"
)

// The type of builder being used.
type BuilderFlowUnion struct {
	BuilderFlowClass *BuilderFlowClass
	Enum             *BuilderFlowEnum
}

func (x *BuilderFlowUnion) UnmarshalJSON(data []byte) error {
	x.BuilderFlowClass = nil
	x.Enum = nil
	var c BuilderFlowClass
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, true)
	if err != nil {
		return err
	}
	if object {
		x.BuilderFlowClass = &c
	}
	return nil
}

func (x *BuilderFlowUnion) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.BuilderFlowClass != nil, x.BuilderFlowClass, false, nil, x.Enum != nil, x.Enum, true)
}

type BasePath struct {
	IntegerArray []int64
	String       *string
}

func (x *BasePath) UnmarshalJSON(data []byte) error {
	x.IntegerArray = nil
	object, err := unmarshalUnion(data, nil, nil, nil, &x.String, true, &x.IntegerArray, false, nil, false, nil, false, nil, true)
	if err != nil {
		return err
	}
	if object {
	}
	return nil
}

func (x *BasePath) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, x.String, x.IntegerArray != nil, x.IntegerArray, false, nil, false, nil, false, nil, true)
}

func unmarshalUnion(data []byte, pi **int64, pf **float64, pb **bool, ps **string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) (bool, error) {
	if pi != nil {
		*pi = nil
	}
	if pf != nil {
		*pf = nil
	}
	if pb != nil {
		*pb = nil
	}
	if ps != nil {
		*ps = nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return false, err
	}

	switch v := tok.(type) {
	case json.Number:
		if pi != nil {
			i, err := v.Int64()
			if err == nil {
				*pi = &i
				return false, nil
			}
		}
		if pf != nil {
			f, err := v.Float64()
			if err == nil {
				*pf = &f
				return false, nil
			}
			return false, errors.New("Unparsable number")
		}
		return false, errors.New("Union does not contain number")
	case float64:
		return false, errors.New("Decoder should not return float64")
	case bool:
		if pb != nil {
			*pb = &v
			return false, nil
		}
		return false, errors.New("Union does not contain bool")
	case string:
		if haveEnum {
			return false, json.Unmarshal(data, pe)
		}
		if ps != nil {
			*ps = &v
			return false, nil
		}
		return false, errors.New("Union does not contain string")
	case nil:
		if nullable {
			return false, nil
		}
		return false, errors.New("Union does not contain null")
	case json.Delim:
		if v == '{' {
			if haveObject {
				return true, json.Unmarshal(data, pc)
			}
			if haveMap {
				return false, json.Unmarshal(data, pm)
			}
			return false, errors.New("Union does not contain object")
		}
		if v == '[' {
			if haveArray {
				return false, json.Unmarshal(data, pa)
			}
			return false, errors.New("Union does not contain array")
		}
		return false, errors.New("Cannot handle delimiter")
	}
	return false, errors.New("Cannot unmarshal union")
}

func marshalUnion(pi *int64, pf *float64, pb *bool, ps *string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) ([]byte, error) {
	if pi != nil {
		return json.Marshal(*pi)
	}
	if pf != nil {
		return json.Marshal(*pf)
	}
	if pb != nil {
		return json.Marshal(*pb)
	}
	if ps != nil {
		return json.Marshal(*ps)
	}
	if haveArray {
		return json.Marshal(pa)
	}
	if haveObject {
		return json.Marshal(pc)
	}
	if haveMap {
		return json.Marshal(pm)
	}
	if haveEnum {
		return json.Marshal(pe)
	}
	if nullable {
		return json.Marshal(nil)
	}
	return nil, errors.New("Union must not be null")
}
