use std::{io::Cursor, sync::Arc};

use c2pa::Builder;
use c2pa::CallbackSigner;
use c2pa::Reader;
use c2pa::settings::Settings;

use c2pa::Ingredient;
use c2pa::jumbf_io;
use c2pa::status_tracker::StatusTracker;
use c2pa::store::Store;

use crate::streams::Stream;
use crate::streams::StreamAdapter;

use serde_json;
use tracing::info;

use crate::error::SPError;
#[uniffi::export]
pub fn get_manifest_and_cert(data: &dyn Stream) -> Result<String, SPError> {
    let reader = Reader::from_stream("video/mp4", StreamAdapter::from(data))
        .map_err(|e| SPError::C2paError(e.to_string()))?;

    if let Some(manifest) = reader.active_manifest() {
        let cert_chain = if let Some(si) = manifest.signature_info() {
            si.cert_chain()
        } else {
            return Err(SPError::NoCertificateChainFound);
        };

        let result = serde_json::json!({
            "manifest": manifest,
            "cert": cert_chain
        });

        return Ok(result.to_string());
    }
    Err(SPError::NoCertificateChainFound)
}

#[uniffi::export(with_foreign)]
pub trait GoSigner: Send + Sync {
    fn sign(&self, data: Vec<u8>) -> Result<Vec<u8>, SPError>;
}

// #[derive(uniffi::Object)]
// struct Authenticator {
//     gosigner: Arc<dyn GoSigner>,
// }

// impl Authenticator {
//     pub fn new(gosigner: Arc<dyn GoSigner>) -> Self {
//         Self { gosigner }
//     }

//     pub fn login(&self) {
//         let username = self.gosigner.get("username".into());
//         let password = self.gosigner.get("password".into());
//     }
// }

const TOML_SETTINGS: &str = r#"
version_major = 1
version_minor = 0

[trust]

[core]
debug = true
hash_alg = "sha256"
salt_jumbf_boxes = true
prefer_box_hash = false
merkle_tree_max_proofs = 5
compress_manifests = true

[verify]
verify_after_reading = false
verify_after_sign = false
verify_trust = false
verify_timestamp_trust = false
ocsp_fetch = false
remote_manifest_fetch = false
check_ingredient_trust = false
skip_ingredient_conflict_resolution = false
strict_v1_validation = false

[builder.thumbnail]
enabled = false
ignore_errors = true
long_edge = 1024
prefer_smallest_format = true
quality = "medium"

[builder.actions]
all_actions_included = false

[builder.actions.auto_created_action]
enabled = true
source_type = "http://c2pa.org/digitalsourcetype/empty"

[builder.actions.auto_opened_action]
enabled = true

[builder.actions.auto_placed_action]
enabled = true
"#;

#[uniffi::export]
pub fn sign(
    manifest: String,
    data: &dyn Stream,
    certs: Vec<u8>,
    gosigner: Arc<dyn GoSigner>,
) -> Result<Vec<u8>, SPError> {
    Settings::from_toml(TOML_SETTINGS).map_err(|e| SPError::C2paError(e.to_string()))?;
    let callback_signer = CallbackSigner::new(
        move |_context: *const (), data: &[u8]| {
            let signature = gosigner
                .sign(data.to_vec())
                .map_err(|e| c2pa::Error::BadParam(e.to_string()))?;
            Ok(signature)
        },
        c2pa::SigningAlg::Es256K,
        certs,
    );
    let mut builder =
        Builder::from_json(&manifest).map_err(|e| SPError::C2paError(e.to_string()))?;
    let mut output = Vec::new();
    let mut input_cursor = StreamAdapter::from(data);
    let mut output_cursor = Cursor::new(&mut output);
    builder
        .sign(
            &callback_signer,
            "video/mp4",
            &mut input_cursor,
            &mut output_cursor,
        )
        .map_err(|e| SPError::C2paError(e.to_string()))?;
    Ok(output)
}

#[uniffi::export]
pub fn get_manifests(data: &dyn Stream) -> Result<String, SPError> {
    let store = Reader::from_stream("video/mp4", StreamAdapter::from(data))
        .map_err(|e| SPError::C2paError(e.to_string()))?;
    let mut certs: std::collections::HashMap<String, String> = std::collections::HashMap::new();
    for (label, manifest) in store.manifests() {
        let cert_chain = manifest
            .signature_info()
            .ok_or(SPError::C2paError(format!(
                "No signature info for manifest: {}",
                label
            )))?
            .cert_chain();
        certs.insert(label.clone(), cert_chain.to_string());
    }
    let result = serde_json::json!({
        "manifests": store.manifests(),
        "certs": certs
    });
    Ok(result.to_string())
}

#[uniffi::export]
pub fn resign(
    unsigned_seg_data: Vec<Vec<u8>>,
    signed_concat_data: &dyn Stream,
    manifest_list: Vec<String>,
    certs: Vec<Vec<u8>>,
) -> Result<Vec<Vec<u8>>, SPError> {
    let mut validation_log = StatusTracker::default();

    let combined_store = Store::from_stream(
        "video/mp4",
        StreamAdapter::from(signed_concat_data),
        true,
        &mut validation_log,
    )
    .map_err(|e| SPError::C2paError(format!("from_stream failed: {}", e)))?;

    // let seg_claim = combined_store
    //     .get_claim(unsigned_seg_label.as_str())
    //     .ok_or(SPError::C2paError(format!(
    //         "Segment claim not found: {}",
    //         unsigned_seg_label
    //     )))?;
    let mut outputs = Vec::<Vec<u8>>::new();

    for (i, manifest_id) in manifest_list.iter().enumerate() {
        let mut output = Vec::<u8>::new();
        let seg_claim =
            combined_store
                .get_claim(manifest_id.as_str())
                .ok_or(SPError::C2paError(format!(
                    "Segment claim not found: {}",
                    manifest_id
                )))?;
        let signature_val = seg_claim.signature_val().clone();

        let callback_signer = CallbackSigner::new(
            move |_context: *const (), _data: &[u8]| Ok(signature_val.clone()),
            c2pa::SigningAlg::Es256K,
            certs[i].clone(),
        );

        let mut seg_store = Store::new();
        let _provenance = seg_store
            .commit_claim(seg_claim.clone())
            .map_err(|e| SPError::C2paError(format!("commit_claim failed: {}", e)))?;

        let mut output_cursor = Cursor::new(&mut output);
        let mut input_cursor = Cursor::new(unsigned_seg_data[i].clone());

        let jumbf_bytes = seg_store
            .to_jumbf(&callback_signer)
            .map_err(|e| SPError::C2paError(format!("to_jumbf failed: {}", e)))?;

        jumbf_io::save_jumbf_to_stream(
            "video/mp4",
            &mut input_cursor,
            &mut output_cursor,
            &jumbf_bytes,
        )
        .map_err(|e| SPError::C2paError(format!("save_jumbf_to_stream failed: {}", e)))?;
        outputs.push(output);
    }

    Ok(outputs)
}

#[uniffi::export]
pub fn sign_with_ingredients(
    manifest: String,
    data: &dyn Stream,
    certs: Vec<u8>,
    ingredients: Vec<Vec<u8>>,
    gosigner: Arc<dyn GoSigner>,
    output: &dyn Stream,
) -> Result<(), SPError> {
    Settings::from_toml(TOML_SETTINGS).map_err(|e| SPError::C2paError(e.to_string()))?;
    let callback_signer = CallbackSigner::new(
        move |_context: *const (), data: &[u8]| {
            gosigner
                .sign(data.to_vec())
                .map_err(|e| c2pa::Error::BadParam(e.to_string()))
        },
        c2pa::SigningAlg::Es256K,
        certs,
    );
    let mut builder =
        Builder::from_json(&manifest).map_err(|e| SPError::C2paError(e.to_string()))?;
    for (i, ingredient) in ingredients.into_iter().enumerate() {
        let mut cursor = Cursor::new(ingredient);
        let ingredient = Ingredient::from_stream("video/mp4", &mut cursor)
            .map_err(|e| SPError::C2paError(e.to_string()))?;
        builder.add_ingredient(ingredient);
    }
    let mut input_cursor = StreamAdapter::from(data);
    let mut output_cursor = StreamAdapter::from(output);
    builder
        .sign(
            &callback_signer,
            "video/mp4",
            &mut input_cursor,
            &mut output_cursor,
        )
        .map_err(|e| SPError::C2paError(e.to_string()))?;
    Ok(())
}
