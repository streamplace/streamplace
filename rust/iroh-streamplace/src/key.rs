use std::str::FromStr;

use crate::error::{Error, InvalidPublicKeySnafu};

/// A public key.
///
/// The key itself is just a 32 byte array, but a key has associated crypto
/// information that is cached for performance reasons.
#[derive(Debug, Clone, Eq, uniffi::Object)]
#[uniffi::export(Display)]
pub struct PublicKey {
    pub(crate) key: [u8; 32],
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
    pub fn to_bytes(&self) -> Vec<u8> {
        self.key.to_vec()
    }

    /// Make a PublicKey from base32 string
    #[uniffi::constructor]
    pub fn from_string(s: String) -> Result<Self, Error> {
        let key = iroh::PublicKey::from_str(&s).map_err(|_| InvalidPublicKeySnafu.build())?;
        Ok(key.into())
    }

    /// Make a PublicKey from byte array
    #[uniffi::constructor]
    pub fn from_bytes(bytes: Vec<u8>) -> Result<Self, Error> {
        if bytes.len() != 32 {
            InvalidPublicKeySnafu.fail()?;
        }
        let bytes: [u8; 32] = bytes.try_into().expect("checked above");
        let key = iroh::PublicKey::from_bytes(&bytes).map_err(|_| InvalidPublicKeySnafu.build())?;
        Ok(key.into())
    }

    /// Convert to a base32 string limited to the first 10 bytes for a friendly string
    /// representation of the key.
    pub fn fmt_short(&self) -> String {
        iroh::PublicKey::from(self).fmt_short()
    }
}

impl PartialEq for PublicKey {
    fn eq(&self, other: &PublicKey) -> bool {
        self.key == other.key
    }
}

impl std::fmt::Display for PublicKey {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        iroh::PublicKey::from(self).fmt(f)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_public_key() {
        let key_str =
            String::from("523c7996bad77424e96786cf7a7205115337a5b4565cd25506a0f297b191a5ea");
        let fmt_str = String::from("523c7996ba");
        let bytes = b"\x52\x3c\x79\x96\xba\xd7\x74\x24\xe9\x67\x86\xcf\x7a\x72\x05\x11\x53\x37\xa5\xb4\x56\x5c\xd2\x55\x06\xa0\xf2\x97\xb1\x91\xa5\xea";
        //
        // create key from string
        let key = PublicKey::from_string(key_str.clone()).unwrap();
        //
        // test methods are as expected
        assert_eq!(key_str, key.to_string());
        assert_eq!(bytes.to_vec(), key.to_bytes());
        assert_eq!(fmt_str, key.fmt_short());
        //
        // create key from bytes
        let key_0 = PublicKey::from_bytes(bytes.to_vec()).unwrap();
        //
        // test methods are as expected
        assert_eq!(key_str, key_0.to_string());
        assert_eq!(bytes.to_vec(), key_0.to_bytes());
        assert_eq!(fmt_str, key_0.fmt_short());
        //
        // test that the eq function works
        assert!(key.equal(&key_0));
        assert!(key_0.equal(&key));
    }
}
