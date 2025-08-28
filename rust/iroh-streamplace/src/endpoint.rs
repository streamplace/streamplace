use iroh::Watcher;

use crate::error::Error;
use crate::utils::NodeAddr;

#[derive(uniffi::Object, Debug, Clone)]
pub struct Endpoint {
    pub(crate) endpoint: iroh::Endpoint,
}

#[uniffi::export]
impl Endpoint {
    /// Create a new endpoint, given a hex-encoded 32-byte ed25519 secret key.
    #[uniffi::constructor(async_runtime = "tokio")]
    pub async fn new(secret_key: &str) -> Result<Self, Error> {
        let secret_key_bytes = hex::decode(secret_key).map_err(|_| Error::InvalidPublicKey)?;

        if secret_key_bytes.len() != 32 {
            return Err(Error::InvalidPublicKey);
        }

        let mut key_array = [0u8; 32];
        key_array.copy_from_slice(&secret_key_bytes);
        let secret_key = iroh::SecretKey::from_bytes(&key_array);
        let endpoint = iroh::Endpoint::builder()
            .secret_key(secret_key)
            .discovery_n0()
            .discovery_local_network()
            .bind()
            .await?;

        Ok(Self { endpoint })
    }

    #[uniffi::method(async_runtime = "tokio")]
    pub async fn node_addr(&self) -> NodeAddr {
        let _ = self.endpoint.home_relay().initialized().await;
        let addr = self.endpoint.node_addr().initialized().await;
        addr.into()
    }
}
