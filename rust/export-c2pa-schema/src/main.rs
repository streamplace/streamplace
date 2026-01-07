use std::{fs, path::Path};

use anyhow::Result;
use c2pa::{Builder, ManifestDefinition, Reader};
use schemars::{schema::RootSchema, schema_for, JsonSchema};
use serde::{Deserialize, Serialize};
use serde_with::skip_serializing_none;

fn write_schema(schema: &RootSchema, name: &str) {
    println!("Exporting JSON schema for {name}");
    let output = serde_json::to_string_pretty(schema).expect("Failed to serialize schema");
    let output_dir = Path::new("./target/schema");
    fs::create_dir_all(output_dir).expect("Could not create schema directory");
    let output_path = output_dir.join(format!("{name}.schema.json"));
    fs::write(&output_path, output).expect("Unable to write schema");
    println!("Wrote schema to {}", output_path.display());
}

#[skip_serializing_none]
#[derive(Debug, Default, Deserialize, Serialize, JsonSchema)]
pub struct Everything {
    pub builder: Builder,
    pub manifest_definition: ManifestDefinition,
    pub reader: Reader,
}

fn main() -> Result<()> {
    let everything = schema_for!(Everything);
    write_schema(&everything, "C2PA");

    Ok(())
}
