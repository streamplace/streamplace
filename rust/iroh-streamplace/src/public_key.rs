use std::{fmt, str::FromStr};

use snafu::Snafu;

#[derive(Debug, Snafu, uniffi::Error)]
#[snafu(module)]
pub enum PublicKeyError {
    Length { size: u64 },
    Invalid { message: String },
}

/// A public key.
///
/// The key itself is just a 32 byte array, but a key has associated crypto
/// information that is cached for performance reasons.
#[derive(Clone, Copy, Eq, Ord, PartialOrd, uniffi::Object)]
#[uniffi::export(Display)]
pub struct PublicKey {
    pub(crate) key: [u8; 32],
}

impl fmt::Debug for PublicKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        iroh::PublicKey::from(self).fmt(f)
    }
}

impl From<iroh::PublicKey> for PublicKey {
    fn from(key: iroh::PublicKey) -> Self {
        PublicKey {
            key: *key.as_bytes(),
        }
    }
}

impl From<&PublicKey> for iroh::PublicKey {
    fn from(key: &PublicKey) -> Self {
        iroh::PublicKey::from_bytes(&key.key).unwrap()
    }
}

#[uniffi::export]
impl PublicKey {
    /// Returns true if the PublicKeys are equal
    pub fn equal(&self, other: &PublicKey) -> bool {
        *self == *other
    }

    /// Express the PublicKey as a byte array
    pub fn as_vec(&self) -> Vec<u8> {
        self.key.to_vec()
    }

    /// Make a PublicKey from base32 string
    #[uniffi::constructor]
    #[allow(clippy::result_large_err)]
    pub fn from_string(s: String) -> Result<Self, PublicKeyError> {
        if s.len() != 64 {
            return Err(PublicKeyError::Length {
                size: s.len() as u64,
            });
        }
        let key = iroh::PublicKey::from_str(&s).map_err(|e| PublicKeyError::Invalid {
            message: e.to_string(),
        })?;
        Ok(key.into())
    }

    /// Make a PublicKey from byte array
    #[uniffi::constructor]
    #[allow(clippy::result_large_err)]
    pub fn from_bytes(bytes: Vec<u8>) -> Result<Self, PublicKeyError> {
        if bytes.len() != 32 {
            return Err(PublicKeyError::Length {
                size: bytes.len() as u64,
            });
        }
        let bytes: [u8; 32] = bytes.try_into().expect("checked above");
        let key = iroh::PublicKey::from_bytes(&bytes).map_err(|e| PublicKeyError::Invalid {
            message: e.to_string(),
        })?;
        Ok(key.into())
    }

    /// Convert to a base32 string limited to the first 10 bytes for a friendly string
    /// representation of the key.
    pub fn fmt_short(&self) -> String {
        iroh::PublicKey::from(self).fmt_short().to_string()
    }
}

impl PartialEq for PublicKey {
    fn eq(&self, other: &PublicKey) -> bool {
        self.key == other.key
    }
}

impl fmt::Display for PublicKey {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        iroh::PublicKey::from(self).fmt(f)
    }
}
